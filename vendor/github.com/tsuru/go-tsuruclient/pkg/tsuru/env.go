/*
 * Tsuru
 *
 * Open source, extensible and Docker-based Platform as a Service (PaaS)
 *
 * API version: 1.6
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package tsuru

// Environment variable.
type Env struct {
	Name string `json:"name,omitempty"`

	Value string `json:"value,omitempty"`
}
