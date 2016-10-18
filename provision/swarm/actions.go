// Copyright 2016 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swarm

import (
	"sort"

	"github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/tsuru/tsuru/action"
	"github.com/tsuru/tsuru/app/image"
	"github.com/tsuru/tsuru/log"
	"github.com/tsuru/tsuru/provision"
	"github.com/tsuru/tsuru/set"
)

type pipelineArgs struct {
	client         *docker.Client
	app            provision.App
	newImage       string
	newImgData     *image.ImageMetadata
	currentImage   string
	currentImgData *image.ImageMetadata
}

func rollbackAddedProcesses(args *pipelineArgs, processes []string) {
	for _, processName := range processes {
		var err error
		if _, in := args.currentImgData.Processes[processName]; in {
			err = deploy(args.client, args.app, processName, args.currentImage)
		} else {
			err = removeService(args.client, args.app, processName)
		}
		if err != nil {
			log.Errorf("error rolling back updated service for %s[%s]: %+v", args.app.GetName(), processName, err)
		}
	}
}

var updateServices = &action.Action{
	Name: "update-services",
	Forward: func(ctx action.FWContext) (action.Result, error) {
		args := ctx.Params[0].(*pipelineArgs)
		var (
			toDeployProcesses []string
			deployedProcesses []string
			err               error
		)
		for processName := range args.newImgData.Processes {
			toDeployProcesses = append(toDeployProcesses, processName)
		}
		sort.Strings(toDeployProcesses)
		for _, processName := range toDeployProcesses {
			err = deploy(args.client, args.app, processName, args.newImage)
			if err != nil {
				break
			}
			deployedProcesses = append(deployedProcesses, processName)
		}
		if err != nil {
			rollbackAddedProcesses(args, deployedProcesses)
			return nil, err
		}
		return deployedProcesses, nil
	},
	Backward: func(ctx action.BWContext) {
		args := ctx.Params[0].(*pipelineArgs)
		deployedProcesses := ctx.FWResult.([]string)
		rollbackAddedProcesses(args, deployedProcesses)
	},
}

var updateImageInDB = &action.Action{
	Name: "update-image-in-db",
	Forward: func(ctx action.FWContext) (action.Result, error) {
		args := ctx.Params[0].(*pipelineArgs)
		err := image.AppendAppImageName(args.app.GetName(), args.newImage)
		if err != nil {
			return nil, errors.Wrap(err, "")
		}
		return ctx.Previous, nil
	},
}

var removeOldServices = &action.Action{
	Name: "remove-old-services",
	Forward: func(ctx action.FWContext) (action.Result, error) {
		args := ctx.Params[0].(*pipelineArgs)
		old := set.FromMap(args.currentImgData.Processes)
		new := set.FromMap(args.newImgData.Processes)
		for processName := range old.Difference(new) {
			err := removeService(args.client, args.app, processName)
			if err != nil {
				log.Errorf("ignored error removing unwanted service for %s[%s]: %+v", args.app.GetName(), processName, err)
			}
		}
		return nil, nil
	},
}