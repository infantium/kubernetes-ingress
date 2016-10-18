package nginx

// Config holds NGINX configuration parameters
type Config struct {
	ProxyConnectTimeout           string
	ProxyReadTimeout              string
	ClientMaxBodySize             string
	MainServerNamesHashBucketSize string
	MainServerWorkerProcesses     string
	MainServerWorkerConnections     string
	MainServerWorkerRLimitNofile     string
	MainServerKeepaliveTimeout     string
	MainServerNamesHashMaxSize    string
}

// NewDefaultConfig creates a Config with default values
func NewDefaultConfig() *Config {
	return &Config{
		ProxyConnectTimeout:        "60s",
		ProxyReadTimeout:           "60s",
		ClientMaxBodySize:          "1m",
		MainServerNamesHashMaxSize: "512",
		MainServerWorkerProcesses:  "auto",
		MainServerWorkerConnections:  "1024",
		MainServerWorkerRLimitNofile:  "2048",
		MainServerKeepaliveTimeout:  "65",
	}
}
