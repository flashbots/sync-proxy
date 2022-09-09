package main

import (
	"flag"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	version = "dev" // is set during build process

	// Default values
	defaultLogLevel       = getEnv("LOG_LEVEL", "info")
	defaultLogJSON        = os.Getenv("LOG_JSON") != ""
	defaultListenAddr     = getEnv("PROXY_LISTEN_ADDR", "localhost:25590")
	defaultTimeoutMs      = getEnvInt("BUILDER_TIMEOUT_MS", 2000) // timeout for all the requests to the builders
	defaultBeaconExpiryMs = getEnvInt("BEACON_EXPIRY_MS", 12000)   // we should be getting requests every slot (12 seconds)

	// Flags
	logJSON          = flag.Bool("json", defaultLogJSON, "log in JSON format instead of text")
	logLevel         = flag.String("loglevel", defaultLogLevel, "log-level: trace, debug, info, warn/warning, error, fatal, panic")
	listenAddr       = flag.String("addr", defaultListenAddr, "listen-address for builder proxy server")
	builderURLs      = flag.String("builders", "", "builder urls - single entry or comma-separated list (scheme://host)")
	builderTimeoutMs = flag.Int("request-timeout", defaultTimeoutMs, "timeout for requests to a builder [ms]")
	proxyURLs        = flag.String("proxies", "", "proxy urls - other proxies to forward BN requests to (scheme://host)")
	proxyTimeoutMs   = flag.Int("proxy-request-timeout", defaultTimeoutMs, "timeout for redundant beacon node requests to another proxy [ms]")
)

var log = logrus.WithField("module", "sync-proxy")

func main() {
	flag.Parse()
	logrus.SetOutput(os.Stdout)

	if *logJSON {
		log.Logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})

	}

	if *logLevel != "" {
		lvl, err := logrus.ParseLevel(*logLevel)
		if err != nil {
			log.Fatalf("Invalid loglevel: %s", *logLevel)
		}
		logrus.SetLevel(lvl)
	}

	log.Infof("sync-proxy %s", version)

	builders := parseURLs(*builderURLs)
	if len(builders) == 0 {
		log.Fatal("No builder urls specified")
	}
	log.WithField("builders", builders).Infof("using %d builders", len(builders))

	builderTimeout := time.Duration(*builderTimeoutMs) * time.Millisecond

	proxies := parseURLs(*proxyURLs)
	log.WithField("proxies", proxies).Infof("using %d proxies", len(proxies))

	proxyTimeout := time.Duration(*proxyTimeoutMs) * time.Millisecond
	beaconExpiry := time.Duration(defaultBeaconExpiryMs) * time.Millisecond

	// Create a new proxy service.
	opts := ProxyServiceOpts{
		ListenAddr:        *listenAddr,
		Builders:          builders,
		BuilderTimeout:    builderTimeout,
		Proxies:           proxies,
		ProxyTimeout:      proxyTimeout,
		BeaconEntryExpiry: beaconExpiry,
		Log:               log,
	}

	proxyService, err := NewProxyService(opts)
	if err != nil {
		log.WithError(err).Fatal("failed creating the server")
	}

	log.Println("listening on", *listenAddr)
	log.Fatal(proxyService.StartHTTPServer())
}

func getEnv(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, ok := os.LookupEnv(key); ok {
		val, err := strconv.Atoi(value)
		if err == nil {
			return val
		}
	}
	return defaultValue
}

func parseURLs(urls string) []*url.URL {
	ret := []*url.URL{}
	for _, entry := range strings.Split(urls, ",") {
		rawURL := strings.TrimSpace(entry)
		if rawURL == "" {
			continue
		}

		// Add protocol scheme prefix if it does not exist.
		if !strings.HasPrefix(rawURL, "http") {
			rawURL = "http://" + rawURL
		}

		// Parse the provided URL.
		url, err := url.ParseRequestURI(rawURL)
		if err != nil {
			log.WithError(err).WithField("url", entry).Fatal("Invalid URL")
		}

		ret = append(ret, url)
	}
	return ret
}
