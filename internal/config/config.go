package config

import (
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	//	  "github.com/gdexlab/go-render/render"
)

type InvalidAddress int64

const (
	IgnoreInvalidAddress  InvalidAddress = 0
	ProcessInvalidAddress InvalidAddress = 1
	DiscardInvalidAddress InvalidAddress = 2
)

type TranslationPrefix struct {
	Prefix  string   `yaml:"prefix"`
	Domains []string `yaml:"domains"`
}

type TranslationEntry struct {
	Domain string
	Prefix net.IP
}

type Translation struct {
	Default  string              `yaml:"default"`
	Prefixes []TranslationPrefix `yaml:"prefixes"`

	DefaultIP net.IP
	Entries   []TranslationEntry
}

type ForwarderZone struct {
	Name      string   `yaml:"name"`
	Upstreams []string `yaml:"upstreams"`
}

type Forwarders struct {
	Default []string        `yaml:"default"`
	Zones   []ForwarderZone `yaml:"zones"`
}

type Cache struct {
	ExpTime   time.Duration `yaml:"expiration"`
	PurgeTime time.Duration `yaml:"purge"`
}

type IPNet struct {
	net.IPNet
}

type Config struct {
	Listen      string            `yaml:"listen"`
	MeshPrefix  IPNet             `yaml:"mesh-prefix"`
	Translation Translation       `yaml:"translation"`
	Forwarders  Forwarders        `yaml:"forwarders"`
	IA          InvalidAddress    `yaml:"invalid-address"`
	Static      map[string]string `yaml:"static"`
	Cache       Cache             `yaml:"cache"`
	LogLevel    string            `yaml:"log-level"`
	StrictIPv6  bool              `yaml:"strict-ipv6"`
	FallBack    bool              `yaml:"allow-fallback-aaaa"`
}

func (a InvalidAddress) String() string {
	switch a {
	case IgnoreInvalidAddress:
		return "Ignore"
	case ProcessInvalidAddress:
		return "Process"
	case DiscardInvalidAddress:
		return "Discard"
	}
	return "Ignore"
}

func (t *Translation) GetPrefix(domain string) net.IP {
	domainLower := strings.ToLower(domain)

	for _, e := range t.Entries {
		if strings.HasSuffix(domainLower, e.Domain) {
			return e.Prefix
		}
	}

	return t.DefaultIP
}

func (t *Translation) Normalize() error {
	t.DefaultIP = net.ParseIP(t.Default)
	if t.DefaultIP == nil {
		return fmt.Errorf("translation: invalid default prefix %q", t.Default)
	}

	t.Entries = nil
	for _, tp := range t.Prefixes {
		prefix := net.ParseIP(tp.Prefix)
		if prefix == nil {
			return fmt.Errorf("translation: invalid prefix %q", tp.Prefix)
		}
		for _, domain := range tp.Domains {
			d := strings.ToLower(domain)
			if !strings.HasSuffix(d, ".") {
				d += "."
			}
			t.Entries = append(t.Entries, TranslationEntry{Domain: d, Prefix: prefix})
		}
	}

	sort.Slice(t.Entries, func(i, j int) bool {
		return len(t.Entries[i].Domain) > len(t.Entries[j].Domain)
	})
	return nil
}

func (f *Forwarders) Normalize() {
	for i := range f.Zones {
		name := strings.ToLower(f.Zones[i].Name)
		if !strings.HasSuffix(name, ".") {
			name += "."
		}
		f.Zones[i].Name = name
	}

	sort.Slice(f.Zones, func(i, j int) bool {
		return len(f.Zones[i].Name) > len(f.Zones[j].Name)
	})
}

func (ia *InvalidAddress) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {
	var IA string

	err = unmarshal(&IA)
	if err != nil {
		return
	}

	switch strings.ToLower(IA) {
	case "ignore":
		*ia = IgnoreInvalidAddress
	case "process":
		*ia = ProcessInvalidAddress
	case "discard":
		*ia = DiscardInvalidAddress
	default:
		return fmt.Errorf("invalid-address must be one of 'ignore/process/discard'")
	}

	return nil
}

func (n *IPNet) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", s, err)
	}
	*n = IPNet{*ipnet}
	return nil
}

func InitConfig() (Config, error) {
	fileName := flag.String("file", "config.yml", "config filename")
	flag.Parse()

	Configs, err := parseFile(*fileName)
	if err != nil {
		return Config{}, err
	}
	return *Configs, nil
}

func parseFile(filePath string) (*Config, error) {
	cfg := new(Config)
	body, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	cfg.Cache.ExpTime = 0
	cfg.Cache.PurgeTime = 0
	cfg.LogLevel = "info"
	cfg.FallBack = false
	_, defaultMesh, _ := net.ParseCIDR("200::/7")
	cfg.MeshPrefix = IPNet{*defaultMesh}
	if err := yaml.UnmarshalStrict(body, &cfg); err != nil {
		return nil, err
	}

	cfg.Forwarders.Normalize()

	if err := cfg.Translation.Normalize(); err != nil {
		return nil, err
	}

	return cfg, nil
}
