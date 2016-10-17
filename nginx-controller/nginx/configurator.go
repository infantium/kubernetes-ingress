package nginx

import (
	"fmt"
	"strings"
	"sync"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

const emptyHost = ""

// Configurator transforms an Ingress resource into NGINX Configuration
type Configurator struct {
	nginx  *NginxController
	config *Config
	lock   sync.Mutex
}

// NewConfigurator creates a new Configurator
func NewConfigurator(nginx *NginxController, config *Config) *Configurator {
	cnf := Configurator{
		nginx:  nginx,
		config: config,
	}

	return &cnf
}

// AddOrUpdateIngress adds or updates NGINX configuration for an Ingress resource
func (cnf *Configurator) AddOrUpdateIngress(name string, ingEx *IngressEx) {
	cnf.lock.Lock()
	defer cnf.lock.Unlock()

	pems := cnf.updateCertificates(ingEx)
	nginxCfg := cnf.generateNginxCfg(ingEx, pems)
	cnf.nginx.AddOrUpdateIngress(name, nginxCfg)
	if err := cnf.nginx.Reload(); err != nil {
		glog.Errorf("Error when adding or updating ingress %q: %q", name, err)
	}
}

func (cnf *Configurator) updateCertificates(ingEx *IngressEx) map[string]string {
	pems := make(map[string]string)

	for _, tls := range ingEx.Ingress.Spec.TLS {
		secretName := tls.SecretName
		secret, exist := ingEx.Secrets[secretName]
		if !exist {
			continue
		}
		cert, ok := secret.Data[api.TLSCertKey]
		if !ok {
			glog.Warningf("Secret %v has no private key", secretName)
			continue
		}
		key, ok := secret.Data[api.TLSPrivateKeyKey]
		if !ok {
			glog.Warningf("Secret %v has no cert", secretName)
			continue
		}

		pemFileName := cnf.nginx.AddOrUpdateCertAndKey(secretName, string(cert), string(key))

		for _, host := range tls.Hosts {
			pems[host] = pemFileName
		}
		if len(tls.Hosts) == 0 {
			pems[emptyHost] = pemFileName
		}
	}

	return pems
}
func (cnf *Configurator) generateNginxCfg(ingEx *IngressEx, pems map[string]string) IngressNginxConfig {
	ingCfg := cnf.createConfig(ingEx)

	upstreams := make(map[string]Upstream)

	wsServices := getWebsocketServices(ingEx)

	if ingEx.Ingress.Spec.Backend != nil {
		name := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)
		upstream := cnf.createUpstream(ingEx, name, ingEx.Ingress.Spec.Backend, ingEx.Ingress.Namespace)
		upstreams[name] = upstream
	}

	var servers []Server

	for _, rule := range ingEx.Ingress.Spec.Rules {
		if rule.IngressRuleValue.HTTP == nil {
			continue
		}

		serverName := rule.Host

		if rule.Host == emptyHost {
			glog.Warningf("Host field of ingress rule in %v/%v is empty", ingEx.Ingress.Namespace, ingEx.Ingress.Name)
		}

		server := Server{Name: serverName}

		if pemFile, ok := pems[serverName]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile
			server.SSLCertificateKey = pemFile
		}

		var locations []Location
		rootLocation := false

		for _, path := range rule.HTTP.Paths {
			upsName := getNameForUpstream(ingEx.Ingress, rule.Host, path.Backend.ServiceName)

			if _, exists := upstreams[upsName]; !exists {
				upstream := cnf.createUpstream(ingEx, upsName, &path.Backend, ingEx.Ingress.Namespace)
				upstreams[upsName] = upstream
			}

			loc := createLocation(pathOrDefault(path.Path), upstreams[upsName], &ingCfg, wsServices[path.Backend.ServiceName])
			locations = append(locations, loc)

			if loc.Path == "/" {
				rootLocation = true
			}
		}

		if rootLocation == false && ingEx.Ingress.Spec.Backend != nil {
			upsName := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)
			loc := createLocation(pathOrDefault("/"), upstreams[upsName], &ingCfg, wsServices[ingEx.Ingress.Spec.Backend.ServiceName])
			locations = append(locations, loc)
		}

		server.Locations = locations
		servers = append(servers, server)
	}

	if len(ingEx.Ingress.Spec.Rules) == 0 && ingEx.Ingress.Spec.Backend != nil {
		server := Server{Name: emptyHost}

		if pemFile, ok := pems[emptyHost]; ok {
			server.SSL = true
			server.SSLCertificate = pemFile
			server.SSLCertificateKey = pemFile
		}

		var locations []Location

		upsName := getNameForUpstream(ingEx.Ingress, emptyHost, ingEx.Ingress.Spec.Backend.ServiceName)

		loc := createLocation(pathOrDefault("/"), upstreams[upsName], &ingCfg, wsServices[ingEx.Ingress.Spec.Backend.ServiceName])
		locations = append(locations, loc)

		server.Locations = locations
		servers = append(servers, server)
	}

	return IngressNginxConfig{Upstreams: upstreamMapToSlice(upstreams), Servers: servers}
}

func (cnf *Configurator) createConfig(ingEx *IngressEx) Config {
	ingCfg := *cnf.config
	if proxyConnectTimeout, exists := ingEx.Ingress.Annotations["nginx.org/proxy-connect-timeout"]; exists {
		ingCfg.ProxyConnectTimeout = proxyConnectTimeout
	}
	if proxyReadTimeout, exists := ingEx.Ingress.Annotations["nginx.org/proxy-read-timeout"]; exists {
		ingCfg.ProxyReadTimeout = proxyReadTimeout
	}
	if clientMaxBodySize, exists := ingEx.Ingress.Annotations["nginx.org/client-max-body-size"]; exists {
		ingCfg.ClientMaxBodySize = clientMaxBodySize
	}

	return ingCfg
}

func getWebsocketServices(ingEx *IngressEx) map[string]bool {
	wsServices := make(map[string]bool)

	if services, exists := ingEx.Ingress.Annotations["nginx.org/websocket-services"]; exists {
		for _, svc := range strings.Split(services, ",") {
			wsServices[svc] = true
		}
	}

	return wsServices
}

func createLocation(path string, upstream Upstream, cfg *Config, websocket bool) Location {
	loc := Location{
		Path:                path,
		Upstream:            upstream,
		ProxyConnectTimeout: cfg.ProxyConnectTimeout,
		ProxyReadTimeout:    cfg.ProxyReadTimeout,
		ClientMaxBodySize:   cfg.ClientMaxBodySize,
		MainServerWorkerProcesses:   cfg.MainServerWorkerProcesses,
		Websocket:           websocket,
	}

	return loc
}

func (cnf *Configurator) createUpstream(ingEx *IngressEx, name string, backend *extensions.IngressBackend, namespace string) Upstream {
	ups := NewUpstreamWithDefaultServer(name)

	endps, exists := ingEx.Endpoints[backend.ServiceName+backend.ServicePort.String()]
	if exists {
		var upsServers []UpstreamServer
		for _, endp := range endps {
			addressport := strings.Split(endp, ":")
			upsServers = append(upsServers, UpstreamServer{addressport[0], addressport[1]})
		}
		if len(upsServers) > 0 {
			ups.UpstreamServers = upsServers
		}
	}

	return ups
}

func pathOrDefault(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func getNameForUpstream(ing *extensions.Ingress, host string, service string) string {
	return fmt.Sprintf("%v-%v-%v-%v", ing.Namespace, ing.Name, host, service)
}

func upstreamMapToSlice(upstreams map[string]Upstream) []Upstream {
	result := make([]Upstream, 0, len(upstreams))

	for _, ups := range upstreams {
		result = append(result, ups)
	}

	return result
}

// DeleteIngress deletes NGINX configuration for an Ingress resource
func (cnf *Configurator) DeleteIngress(name string) {
	cnf.lock.Lock()
	defer cnf.lock.Unlock()

	cnf.nginx.DeleteIngress(name)
	if err := cnf.nginx.Reload(); err != nil {
		glog.Errorf("Error when removing ingress %q: %q", name, err)
	}
}

// UpdateEndpoints updates endpoints in NGINX configuration for an Ingress resource
func (cnf *Configurator) UpdateEndpoints(name string, ingEx *IngressEx) {
	cnf.AddOrUpdateIngress(name, ingEx)
}

// UpdateConfig updates NGINX Configuration parameters
func (cnf *Configurator) UpdateConfig(config *Config) {
	cnf.lock.Lock()
	defer cnf.lock.Unlock()

	cnf.config = config
	mainCfg := &NginxMainConfig{
		MainServerWorkerProcesses: config.MainServerWorkerProcesses,
		ServerNamesHashBucketSize: config.MainServerNamesHashBucketSize,
		ServerNamesHashMaxSize:    config.MainServerNamesHashMaxSize,
	}

	cnf.nginx.UpdateMainConfigFile(mainCfg)
}
