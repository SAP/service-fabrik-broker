/*
Copyright 2018 The Service Fabrik Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator/pkg/internal/renderer"

	"k8s.io/client-go/kubernetes"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	chartapi "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/timeconv"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var ignoreFileSuffix = [...]string{"NOTES.txt"}

type helmRenderer struct {
	renderer     *engine.Engine
	capabilities *chartutil.Capabilities
	clientSet    *kubernetes.Clientset
}

type helmInput struct {
	chartPath   string
	releaseName string
	namespace   string
	values      map[string]interface{}
}

// NewInput creates a new helm Renderer input object.
func NewInput(chartPath, releaseName, namespace string, values map[string]interface{}) renderer.Input {
	return helmInput{
		chartPath:   chartPath,
		releaseName: releaseName,
		namespace:   namespace,
		values:      values,
	}
}

// New creates a new helm Renderer object.
func New(clientSet *kubernetes.Clientset) (renderer.Renderer, error) {
	if clientSet == nil {
		cfg, err := config.GetConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to set up client config for helm chart renderer %v", err)
		}

		clientSet, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes client %v", err)
		}
	}
	sv, err := clientSet.ServerVersion()

	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes server version %v", err)
	}
	return &helmRenderer{
		clientSet:    clientSet,
		renderer:     engine.New(),
		capabilities: &chartutil.Capabilities{KubeVersion: sv},
	}, nil
}

// Render loads the chart from the given location <chartPath> and calls the Render() function
// to convert it into a renderer.Output object.
func (r *helmRenderer) Render(rawInput renderer.Input) (renderer.Output, error) {
	input, ok := rawInput.(helmInput)
	if !ok {
		return nil, fmt.Errorf("invalid input to helm chart renderer")
	}
	chart, err := chartutil.Load(input.chartPath)
	if err != nil {
		return nil, fmt.Errorf("can't create load chart from path %s:, %s", input.chartPath, err)
	}
	return r.renderRelease(chart, input.releaseName, input.namespace, input.values)
}

func (r *helmRenderer) renderRelease(chart *chartapi.Chart, releaseName, namespace string, values map[string]interface{}) (renderer.Output, error) {
	chartName := chart.GetMetadata().GetName()

	parsedValues, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("can't parse variables for chart %s: ,%s", chartName, err)
	}
	chartConfig := &chartapi.Config{Raw: string(parsedValues)}

	err = chartutil.ProcessRequirementsEnabled(chart, chartConfig)
	if err != nil {
		return nil, fmt.Errorf("can't process requirements for chart %s: ,%s", chartName, err)
	}
	err = chartutil.ProcessRequirementsImportValues(chart)
	if err != nil {
		return nil, fmt.Errorf("can't process requirements for import values for chart %s: ,%s", chartName, err)
	}

	caps := r.capabilities
	revision := 1
	ts := timeconv.Now()
	options := chartutil.ReleaseOptions{
		Name:      releaseName,
		Time:      ts,
		Namespace: namespace,
		Revision:  revision,
		IsInstall: true,
	}

	valuesToRender, err := chartutil.ToRenderValuesCaps(chart, chartConfig, options, caps)
	if err != nil {
		return nil, err
	}
	return r.renderResources(chart, valuesToRender)
}

func (r *helmRenderer) renderResources(ch *chartapi.Chart, values chartutil.Values) (renderer.Output, error) {
	files, err := r.renderer.Render(ch, values)
	if err != nil {
		return nil, err
	}

	// Remove NODES.txt and partials
	for k := range files {
		if strings.HasPrefix(path.Base(k), "_") {
			delete(files, k)
			continue
		}
		for _, suffix := range ignoreFileSuffix {
			if strings.HasSuffix(k, suffix) {
				delete(files, k)
				break
			}
		}
	}

	return &helmOutput{
		Name:  ch.Metadata.Name,
		Files: files,
	}, nil
}
