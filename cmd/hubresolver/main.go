/*
Copyright 2022 The Tekton Authors
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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tektoncd/resolution/pkg/common"
	"github.com/tektoncd/resolution/pkg/resolver/framework"
	"knative.dev/pkg/injection/sharedmain"
)

const defaultHubURL = "https://api.hub.tekton.dev/v1/resource/Tekton/%s/%s/%s/yaml"
const yamlEndpoint = "v1/resource/Tekton/%s/%s/%s/yaml"

func main() {
	apiURL := os.Getenv("HUB_API")
	hubURL := defaultHubURL
	if apiURL == "" {
		hubURL = defaultHubURL
	} else {
		if !strings.HasSuffix(apiURL, "/") {
			apiURL = apiURL + "/"
		}
		hubURL = apiURL + yamlEndpoint
	}
	fmt.Println("RUNNING WITH HUB URL PATTERN:", hubURL)
	resolver := resolver{hubURL: hubURL}
	sharedmain.Main("controller",
		framework.NewController(context.Background(), &resolver),
	)
}

type resolver struct {
	hubURL string
}

// Initialize sets up any dependencies needed by the resolver. None atm.
func (r *resolver) Initialize(context.Context) error {
	return nil
}

// GetName returns a string name to refer to this resolver by.
func (r *resolver) GetName(context.Context) string {
	return "Hub"
}

// GetSelector returns a map of labels to match requests to this resolver.
func (r *resolver) GetSelector(context.Context) map[string]string {
	return map[string]string{
		common.LabelKeyResolverType: "hub",
	}
}

// ValidateParams ensures parameters from a request are as expected.
func (r *resolver) ValidateParams(ctx context.Context, params map[string]string) error {
	if kind, ok := params["kind"]; !ok {
		return errors.New("must include kind param")
	} else if kind != "task" && kind != "pipeline" {
		return errors.New("kind param must be task or pipeline")
	}
	if _, ok := params["name"]; !ok {
		return errors.New("must include name param")
	}
	if _, ok := params["version"]; !ok {
		return errors.New("must include version param")
	}
	return nil
}

type dataResponse struct {
	YAML string `json:"yaml"`
}

type hubResponse struct {
	Data dataResponse `json:"data"`
}

// Resolve uses the given params to resolve the requested file or resource.
func (r *resolver) Resolve(ctx context.Context, params map[string]string) (framework.ResolvedResource, error) {
	url := fmt.Sprintf(defaultHubURL, params["kind"], params["name"], params["version"])
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error requesting resource from hub: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	hr := hubResponse{}
	err = json.Unmarshal(body, &hr)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling json response: %w", err)
	}
	return &hubResolvedResource{
		data: []byte(hr.Data.YAML),
	}, nil
}

// myResolvedResource wraps the data we want to return to Pipelines
type hubResolvedResource struct {
	data []byte
}

// Data returns the bytes of our hard-coded Pipeline
func (rr *hubResolvedResource) Data() []byte {
	return rr.data
}

// Annotations returns any metadata needed alongside the data. None atm.
func (*hubResolvedResource) Annotations() map[string]string {
	return nil
}
