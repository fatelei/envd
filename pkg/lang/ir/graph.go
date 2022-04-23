// Copyright 2022 The MIDI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ir

import (
	"context"

	"github.com/moby/buildkit/client/llb"
)

// A Graph contains the state,
// such as its call stack and thread-local storage.
type Graph struct {
	OS             string
	Language       string
	PyPIPackages   []string
	SystemPackages []string
}

func NewGraph() *Graph {
	return &Graph{
		OS:             osDefault,
		Language:       languageDefault,
		PyPIPackages:   []string{},
		SystemPackages: []string{},
	}
}

var DefaultGraph = NewGraph()

func Compile(ctx context.Context) (*llb.Definition, error) {
	state := DefaultGraph.Compile()
	// TODO(gaocegege): Support multi platform.
	def, err := state.Marshal(ctx, llb.LinuxAmd64)
	if err != nil {
		return nil, err
	}
	return def, nil
}

func (g Graph) Compile() llb.State {
	// TODO(gaocegege): Support more OS and langs.
	base := llb.Image("docker.io/library/python:3.8")
	system := g.compileSystemPackages(base)
	pypi := g.compilePyPIPackages(base)
	ssh_stage := g.copyMidiSSHServer(base)
	merged := llb.Merge([]llb.State{
		system, pypi, ssh_stage,
	})
	return merged
}

func (g Graph) compilePyPIPackages(root llb.State) llb.State {
	if len(g.PyPIPackages) == 0 {
		return root
	}
	// TODO(gaocegege): Support per-user config to keep the mirror.
	cmd := "pip install -i https://mirror.sjtu.edu.cn/pypi/web/simple"
	cacheDir := "/root/.cache/pip"
	for _, pkg := range g.PyPIPackages {
		cmd = cmd + " " + pkg
	}
	run := root.Run(llb.Shlex(cmd))
	run.AddMount(cacheDir, llb.Scratch(),
		llb.AsPersistentCacheDir("/"+cacheDir, llb.CacheMountShared))
	return run.Root()
}

func (g Graph) compileSystemPackages(root llb.State) llb.State {
	if len(g.PyPIPackages) == 0 {
		return root
	}
	// TODO(gaocegege): Support per-user config to keep the mirror.
	cmd := "apt install"
	cacheDir := "/var/cache/apt"
	cacheLibDir := "/var/lib/apt"
	for _, pkg := range g.SystemPackages {
		cmd = cmd + " " + pkg
	}
	run := root.Run(llb.Shlex(cmd))
	run.AddMount(cacheDir, llb.Scratch(),
		llb.AsPersistentCacheDir("/"+cacheDir, llb.CacheMountShared))
	run.AddMount(cacheLibDir, llb.Scratch(),
		llb.AsPersistentCacheDir("/"+cacheLibDir, llb.CacheMountShared))
	return run.Root()
}

func (g Graph) copyMidiSSHServer(root llb.State) llb.State {
	run := root.File(llb.Mkdir("/var/midi/remote/", 0700, llb.WithParents(true))).
		File(llb.Mkdir("/var/midi/bin/", 0700, llb.WithParents(true))).
		File(llb.Copy(llb.Local("context"), "examples/ssh_keypairs/public.pub", "/var/midi/remote/authorized_keys")).
		File(llb.Copy(llb.Local("context"), "bin/midi-ssh", "/var/midi/bin/midi-ssh"))
	return run
}