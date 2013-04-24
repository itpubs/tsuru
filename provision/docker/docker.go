// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/globocom/config"
	"github.com/globocom/tsuru/fs"
	"github.com/globocom/tsuru/log"
	"strings"
)

var fsystem fs.Fs

func filesystem() fs.Fs {
	if fsystem == nil {
		fsystem = fs.OsFs{}
	}
	return fsystem
}

// container represents an docker container with the given name.
type container struct {
	name       string
	instanceId string
}

// runCmd executes commands and log the given stdout and stderror.
func runCmd(cmd string, args ...string) (string, error) {
	out := bytes.Buffer{}
	err := executor().Execute(cmd, args, nil, &out, &out)
	log.Printf("running the cmd: %s with the args: %s", cmd, args)
	return out.String(), err
}

// ip returns the ip for the container.
func (c *container) ip() (string, error) {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return "", err
	}
	log.Printf("Getting ipaddress to instance %s", c.instanceId)
	instanceJson, err := runCmd(docker, "inspect", c.instanceId)
	if err != nil {
		msg := "error(%s) trying to inspect docker instance(%s) to get ipaddress"
		log.Printf(msg, err)
		return "", errors.New(msg)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(instanceJson), &result); err != nil {
		msg := "error(%s) parsing json from docker when trying to get ipaddress"
		log.Printf(msg, err)
		return "", errors.New(msg)
	}
	if ns, ok := result["NetworkSettings"]; !ok || ns == nil {
		msg := "Error when getting container information. NetworkSettings is missing."
		log.Printf(msg)
		return "", errors.New(msg)
	}
	networkSettings := result["NetworkSettings"].(map[string]interface{})
	instanceIp := networkSettings["IpAddress"].(string)
	if instanceIp == "" {
		msg := "error: Can't get ipaddress..."
		log.Print(msg)
		return "", errors.New(msg)
	}
	log.Printf("Instance IpAddress: %s", instanceIp)
	return instanceIp, nil
}

// create creates a docker container with base template by default.
func (c *container) create() (string, error) {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return "", err
	}
	template, err := config.GetString("docker:image")
	if err != nil {
		return "", err
	}
	cmd, err := config.GetString("docker:cmd:bin")
	if err != nil {
		return "", err
	}
	args, err := config.GetList("docker:cmd:args")
	if err != nil {
		return "", err
	}
	args = append([]string{"run", "-d", template, cmd}, args...)
	instanceId, err := runCmd(docker, args...)
	instanceId = strings.Replace(instanceId, "\n", "", -1)
	log.Printf("docker instanceId=%s", instanceId)
	return instanceId, err
}

// start starts a docker container.
func (c *container) start() error {
	// it isn't necessary to start a docker container after docker run.
	return nil
}

// stop stops a docker container.
func (c *container) stop() error {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return err
	}
	//TODO: better error handling
	log.Printf("trying to stop instance %s", c.instanceId)
	output, err := runCmd(docker, "stop", c.instanceId)
	log.Printf("docker stop=%s", output)
	return err
}

// destroy destory a docker container.
func (c *container) destroy() error {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return err
	}
	//TODO: better error handling
	//TODO: Remove host's nginx route
	log.Printf("trying to destroy instance %s", c.instanceId)
	_, err = runCmd(docker, "rm", c.instanceId)
	return err
}

func (c *container) commit(imgName string) error {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return err
	}
	registryUser, err := config.GetString("docker:registry-user")
	if err != nil {
		return err
	}
	log.Printf("attempting to commit image from container %s", c.instanceId)
	imgName = fmt.Sprintf("%s/%s", registryUser, imgName)
	_, err = runCmd(docker, "commit", c.instanceId, imgName)
	if err != nil {
		log.Printf("Could not commit docker image: %s", err.Error())
		return err
	}
	return nil
}
