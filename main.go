package main

// k8snodemon - k8s/openstack node mangement tool
//
// Copyright (c) 2021 - Valentin Kuznetsov <vkuznet AT gmail dot com>
//
//

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"

	//     "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	var env bool
	flag.BoolVar(&env, "env", false, "use openstack environment for authentication")
	var k8s bool
	flag.BoolVar(&k8s, "k8s", false, "use k8s APIs to get node information")
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
	if name == "" && token == "" && env == false {
		var err error
		name, password, err = credentials()
		if err != nil {
			log.Fatal("unable to read credentials")
		}
	}
	log.SetFlags(0)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if k8s {
		k8srun(endpoint, token, appid, name, password, project, method, env, verbose)
	} else {
		run(endpoint, token, appid, name, password, project, method, env, verbose)
	}
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
func getClient(endpoint, token, appid, username, password, project, method string, env, verbose bool) (*gophercloud.ServiceClient, error) {

	var opts gophercloud.AuthOptions
	var err error
	scope := &gophercloud.AuthScope{
		ProjectName: project,
		DomainName:  "default",
		DomainID:    "default",
	}
	if appid != "" {
		// another way to make authentication via clientconfig
		// it requires to import
		// "github.com/gophercloud/utils/openstack/clientconfig"
		/*
			copts := &clientconfig.ClientOpts{
				AuthInfo: &clientconfig.AuthInfo{
					AuthURL:                     endpoint,
					ApplicationCredentialID:     appid,
					ApplicationCredentialSecret: password,
					UserDomainID:                "default",
					ProjectDomainID:             "default",
				},
			}
			ao, err := clientconfig.AuthOptions(copts)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("auth options: %+v", *ao)
			log.Printf("scope options: %+v", *ao.Scope)
		*/
		opts = gophercloud.AuthOptions{
			IdentityEndpoint:            endpoint,
			ApplicationCredentialID:     appid,
			ApplicationCredentialSecret: password,
			TenantName:                  project,
			DomainID:                    "default",
			Scope:                       &gophercloud.AuthScope{},
		}
		if verbose {
			log.Printf("auth options: %+v\n", opts)
		}
	} else if token != "" {
		opts = gophercloud.AuthOptions{
			IdentityEndpoint: endpoint,
			TokenID:          token,
			Scope:            scope,
		}
	} else if env {
		ao, err := openstack.AuthOptionsFromEnv()
		if err != nil {
			log.Fatal(err)
		}
		opts = ao
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
	return client, err
}

// helper function to run our workflow on openstack
func run(endpoint, token, appid, username, password, project, method string, env, verbose bool) {
	client, err := getClient(endpoint, token, appid, username, password, project, method, env, verbose)
	if err != nil {
		log.Fatal("compute client error", err)
	}
	// list existing servers
	allPages, err := servers.List(client, nil).AllPages()
	if err != nil {
		log.Fatal(err)
	}
	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range allServers {
		if verbose {
			log.Printf("%+v\n", s)
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

	/*
		log.Println("### old way to list servers")

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
					log.Println(s.ID, s.Name, s.Status)
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
	*/
}

// NodeInfo provides information about k8s nodes
type NodeInfo struct {
	Name   string // k8d node name (github.com/kubernetes/client-go)
	ID     string // k8s node ID (github.com/kubernetes/client-go)
	Status string // status of the node (github.com/kubernetes/apimachinery)
}

// helper function to get k8s nodes
func k8snodes(verbose bool) []NodeInfo {
	var out []NodeInfo
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err.Error())
	}
	// get nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}
	for _, n := range nodes.Items {
		data, err := json.MarshalIndent(n, "", "  ")
		//         log.Printf("node: %+v", n)
		if err == nil {
			if verbose {
				log.Println(string(data))
			}
		}
		// lookup status of the node within status conditions data structure
		var status string
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" {
				status = fmt.Sprintf("%v", c.Status)
				break
			}
		}
		nid := strings.Replace(n.Spec.ProviderID, "openstack:///", "", -1)
		ninfo := NodeInfo{Name: n.Name, ID: nid, Status: status}
		out = append(out, ninfo)
	}
	return out
}

// k8srun helper function to manage nodes within k8s
func k8srun(endpoint, token, appid, username, password, project, method string, env, verbose bool) {
	client, err := getClient(endpoint, token, appid, username, password, project, method, env, verbose)
	if err != nil {
		log.Fatal("compute client error", err)
	}
	log.Println("openstack client", client)
	// list existing servers
	nodes := k8snodes(verbose)
	log.Println("nodes", nodes)
	for _, n := range nodes {
		log.Printf("ID=%s name=%s status=%v\n", n.ID, n.Name, n.Status)
		if n.Status != "True" { // node is not ready
			log.Printf("found %s (%s) with status %s, will apply %s reboot", n.Name, n.ID, n.Status, method)
			var res servers.ActionResult
			if method == "soft" {
				rebootMethod := servers.RebootOpts{Type: servers.SoftReboot}
				res = servers.Reboot(client, n.ID, rebootMethod)
			} else {
				rebootMethod := servers.RebootOpts{Type: servers.HardReboot}
				res = servers.Reboot(client, n.ID, rebootMethod)
			}
			log.Println("result", res)
		}
	}
	/*

		// list existing opentstack servers
		allPages, err := servers.List(client, nil).AllPages()
		if err != nil {
			log.Fatal(err)
		}
		allServers, err := servers.ExtractServers(allPages)
		if err != nil {
			log.Fatal(err)
		}
		for _, s := range allServers {
			//         if verbose {
			//             log.Printf("ID=%s name=%s status=%v\n", s.ID, s.Name, s.Status)
			//         }
			if inList(s.Name, nodes) {
				log.Printf("ID=%s name=%s status=%v\n", s.ID, s.Name, s.Status)
				if verbose {
					log.Printf("%+v\n", s)
				}
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
					log.Println("result", res)
				}
			}
		}
	*/
}

// helper function to check if given value in a list
func inList(s string, list []string) bool {
	for _, v := range list {
		if s == v {
			return true
		}
	}
	return false
}
