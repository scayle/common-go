package common

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/api"
)

func connect() *api.Client {
	config := api.DefaultConfig()
	consulHost := os.Getenv("CONSUL_HOST")
	if consulHost != "" {
		config.Address = consulHost
	}

	consul, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("could not create consul client %v", err)
	}

	return consul
}

type Option func(c *config)

type config struct {
	defaultPort           int
	registrationModifiers []func(*api.AgentServiceRegistration)
}

func defaultConfig() *config {
	cfg := &config{}
	WithDefaultPort(8100)(cfg)
	return cfg
}

// WithDefaultPort sets the default port for the service.
// This setting can always be overwritten by an environment variable named PRODUCT_SERVICE_PORT.
func WithDefaultPort(defaultPort int) Option {
	return func(o *config) {
		o.defaultPort = defaultPort
	}
}

func WithRegistrationModifier(modifier func(*api.AgentServiceRegistration)) Option {
	return func(o *config) {
		o.registrationModifiers = append(o.registrationModifiers, modifier)
	}
}

// WithHTTPHealthCheck enables a health check using a simple small webserver
// which gets automatically started.
// The default port setting can always be overwritten by an environment variable named PRODUCT_HEALTH_PORT.
func WithHTTPHealthCheck(defaultPort int) Option {
	return WithRegistrationModifier(func(registration *api.AgentServiceRegistration) {
		// setup simple health detection using a small webserver
		registration.Check = new(api.AgentServiceCheck)
		registration.Check.HTTP = fmt.Sprintf("http://%s:%d/healthcheck", registration.Address, healthPort(defaultPort))
		registration.Check.Interval = "5s"
		registration.Check.Timeout = "3s"
		http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
			_, err := fmt.Fprintf(w, `I am alive!`)
			if err != nil {
				panic(err)
			}
		})

		go func() {
			err := http.ListenAndServe(fmt.Sprintf(":%d", healthPort(defaultPort)), nil)
			log.Fatalf("healthcheck webserver failed %v", err)
		}()
	})
}

// RegisterConsulService registers a new service to consul and returns the final (already registered) registration.
func RegisterConsulService(serviceName string, options ...Option) *api.AgentServiceRegistration {
	cfg := defaultConfig()
	for _, o := range options {
		o(cfg)
	}

	// connect to consul
	consul := connect()

	// setup registration
	registration := new(api.AgentServiceRegistration)
	registration.ID = Hostname()
	registration.Name = serviceName
	address := Hostname()
	registration.Address = address
	registration.Port = port(cfg.defaultPort)

	for _, m := range cfg.registrationModifiers {
		m(registration)
	}

	// finally register the service
	err := consul.Agent().ServiceRegister(registration)
	if err != nil {
		log.Fatalf("registering to consul failed %v", err)
	}

	return registration
}

// RegisterServiceWithConsul registers a new service to consul.
//
// Deprecated: Use RegisterConsulService. RegisterServiceWithConsul will be removed v1.0.0.
// To use a simple HTTP health service, use WithHTTPHealthCheck.
func RegisterServiceWithConsul(serviceName string) {
	// ToDo: remove on v1.0.0
	RegisterConsulService(serviceName, WithHTTPHealthCheck(8101))
}

// GetRandomServiceWithConsul returns any active service with the given name.
func GetRandomServiceWithConsul(serviceName string) *api.ServiceEntry {
	services := GetServicesWithConsul(serviceName)
	if len(services) == 0 {
		return nil
	}

	return services[rand.Intn(len(services))]
}

// GetServicesWithConsul returns all active services for the given name.
func GetServicesWithConsul(serviceName string) []*api.ServiceEntry {
	consul := connect()

	services, _, err := consul.Health().Service(serviceName, "", true, &api.QueryOptions{})
	if err != nil {
		log.Fatalf("searching for service failed %v", err)
	}

	return services
}

func port(defaultPort int) int {
	p := os.Getenv("PRODUCT_SERVICE_PORT")
	if len(strings.TrimSpace(p)) == 0 {
		return defaultPort
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		panic("invalid format for the environment variable PRODUCT_SERVICE_PORT")
	}
	return port
}

func healthPort(defaultPort int) int {
	p := os.Getenv("PRODUCT_HEALTH_PORT")
	if len(strings.TrimSpace(p)) == 0 {
		return defaultPort
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		panic("invalid format for the environment variable PRODUCT_HEALTH_PORT")
	}
	return port
}

// Deprecation: replaced by port
func Port() string {
	p := os.Getenv("PRODUCT_SERVICE_PORT")
	if len(strings.TrimSpace(p)) == 0 {
		return ":8100"
	}
	return fmt.Sprintf(":%s", p)
}

// Deprecation: replaced by healthPort
func HealthPort() string {
	p := os.Getenv("PRODUCT_HEALTH_PORT")
	if len(strings.TrimSpace(p)) == 0 {
		return ":8101"
	}
	return fmt.Sprintf(":%s", p)
}
