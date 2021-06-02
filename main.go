package main

import (
	"bufio"
	"flag"
	"fmt"
	"golang.org/x/term"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/pagination"
)

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	var token string
	flag.StringVar(&token, "token", "", "token")
	var appid string
	flag.StringVar(&appid, "appid", "", "appid")
	var name string
	flag.StringVar(&name, "name", "", "user or app name")
	var password string
	flag.StringVar(&password, "password", "", "user password or app secret")
	var endpoint string
	flag.StringVar(&endpoint, "endpoint", "", "endpoint")
	var project string
	flag.StringVar(&project, "project", "CMS Web", "project")
	var method string
	flag.StringVar(&method, "method", "soft", "method")
	flag.Parse()
	log.SetFlags(0)
	log.SetFlags(log.Lshortfile)
	if name == "" && token == "" {
		var err error
		name, password, err = credentials()
		if err != nil {
			log.Fatal("unable to read credentials")
		}
	}
	run(endpoint, token, appid, name, password, project, method, verbose)
}

// helper function to get user credentials from stdin
func credentials() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}
	fmt.Println("")

	password := string(bytePassword)
	return strings.TrimSpace(username), strings.TrimSpace(password), nil
}

// helper function to run our workflow on openstack
func run(endpoint, token, appid, username, password, project, method string, verbose bool) {

	var opts gophercloud.AuthOptions
	var err error
	scope := &gophercloud.AuthScope{
		ProjectName: project,
		DomainName:  "default",
		DomainID:    "default",
	}
	if appid != "" {
		opts = gophercloud.AuthOptions{
			IdentityEndpoint:            endpoint,
			ApplicationCredentialID:     appid,
			ApplicationCredentialName:   username,
			ApplicationCredentialSecret: password,
			Scope:                       scope,
		}
		log.Printf("auth options: %+v\n", opts)
	} else if token != "" {
		opts = gophercloud.AuthOptions{
			IdentityEndpoint: endpoint,
			TokenID:          token,
			Scope:            scope,
		}
	} else {
		opts = gophercloud.AuthOptions{
			IdentityEndpoint: endpoint,
			Username:         username,
			Password:         password,
			DomainID:         "default",
			Scope:            scope,
		}
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		log.Fatal("auth client failure: ", err)
	}
	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		log.Fatal("compute client error", err)
	}

	// list existing servers
	pager := servers.List(client, servers.ListOpts{})
	pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := servers.ExtractServers(page)
		if err != nil {
			log.Println("extract servers: ", err)
			return false, err
		}
		for _, s := range serverList {
			if verbose {
				fmt.Println(s.ID, s.Name, s.Status)
			}
			// reboot specific server
			if strings.ToLower(s.Status) != "active" {
				log.Printf("found %s (%s) with status %s, will apply %s reboot", s.Name, s.ID, s.Status, method)
				var res servers.ActionResult
				if method == "soft" {
					rebootMethod := servers.RebootOpts{Type: servers.SoftReboot}
					res = servers.Reboot(client, s.ID, rebootMethod)
				} else {
					rebootMethod := servers.RebootOpts{Type: servers.HardReboot}
					res = servers.Reboot(client, s.ID, rebootMethod)
				}
				log.Println("results", res)
			}
		}
		return true, nil
	})
}
