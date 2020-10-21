package common

import (
	"log"
	"os"
)

func Hostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("retrieving Hostname failed %v", err)
	}
	return hostname
}
