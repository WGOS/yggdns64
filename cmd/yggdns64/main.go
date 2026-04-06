package main

// Based on https://github.com/katakonst/go-dns-proxy/releases

import (
	"log"

	"github.com/WGOS/yggdns64/internal/config"
	"github.com/WGOS/yggdns64/internal/logger"
	"github.com/WGOS/yggdns64/internal/proxy"
	"github.com/miekg/dns"
)

func main() {
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("Failed to load configs: %s", err)
	}

	dnsProxy := proxy.NewProxy(cfg)

	logger := logger.NewLogger(cfg.LogLevel)

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		switch r.Opcode {
		case dns.OpcodeQuery:
			m, err := dnsProxy.GetResponse(r)
			if err != nil {
				logger.Errorf("Failed lookup for %s with error: %s\n", r, err.Error())
			}
			w.WriteMsg(m)
		}
	})

	server := &dns.Server{Addr: cfg.Listen, Net: "udp"}
	logger.Infof("Starting at %s\n", cfg.Listen)
	err = server.ListenAndServe()
	if err != nil {
		logger.Errorf("Failed to start server: %s\n ", err.Error())
	}
}
