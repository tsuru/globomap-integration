/*
 * Tsuru
 *
 * Open source, extensible and Docker-based Platform as a Service (PaaS)
 *
 * API version: 1.6
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package tsuru

// Volume plan.
type VolumePlan struct {

	// Volume plan name.
	Name string `json:"name,omitempty"`

	// Volume plan opts.
	Opts map[string]string `json:"opts,omitempty"`
}
