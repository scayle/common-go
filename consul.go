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

// RegisterServiceWithConsul registers a new service to consul
// and also enables a health check using a simple small webserver
// which gets automatically started.
// To configure the Port you can pass
// PRODUCT_SERVICE_PORT and PRODUCT_HEALTH_PORT as environment variable.
// The default ports are 8100 and 8101.
func RegisterServiceWithConsul(serviceName string) {
	// connect to consul
	consul := connect()

	// setup registration
	registration := new(api.AgentServiceRegistration)
	registration.ID = Hostname()
	registration.Name = serviceName
	address := Hostname()
	registration.Address = address
	port, err := strconv.Atoi(Port()[1:len(Port())])
	if err != nil {
		log.Fatalf("wrong Port format %v", err)
	}
	registration.Port = port

	// setup simple health detection using a small webserver
	registration.Check = new(api.AgentServiceCheck)
	healthPortNr, err := strconv.Atoi(HealthPort()[1:len(HealthPort())])
	if err != nil {
		log.Fatalf("wrong healt Port format %v", err)
	}
	registration.Check.HTTP = fmt.Sprintf("http://%s:%v/healthcheck", address, healthPortNr)
	registration.Check.Interval = "5s"
	registration.Check.Timeout = "3s"
	http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `I am alive!`)
		if err != nil {
			panic(err)
		}
	})

	go func() {
		err := http.ListenAndServe(HealthPort(), nil)
		log.Fatalf("healthcheck webserver failed %v", err)
	}()

	// finally register the service
	err = consul.Agent().ServiceRegister(registration)
	if err != nil {
		log.Fatalf("registering to consul failed %v", err)
	}
}

func GetRandomServiceWithConsul(serviceName string) *api.ServiceEntry {
	services := GetServicesWithConsul(serviceName)
	if len(services) == 0 {
		return nil
	}

	return services[rand.Intn(len(services))]
}

func GetServicesWithConsul(serviceName string) []*api.ServiceEntry {
	consul := connect()

	services, _, err := consul.Health().Service(serviceName, "", true, &api.QueryOptions{})
	if err != nil {
		log.Fatalf("searching for service failed %v", err)
	}

	return services
}

func Port() string {
	p := os.Getenv("PRODUCT_SERVICE_PORT")
	if len(strings.TrimSpace(p)) == 0 {
		return ":8100"
	}
	return fmt.Sprintf(":%s", p)
}

func HealthPort() string {
	p := os.Getenv("PRODUCT_HEALTH_PORT")
	if len(strings.TrimSpace(p)) == 0 {
		return ":8101"
	}
	return fmt.Sprintf(":%s", p)
}
