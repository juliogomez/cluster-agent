package instrumentation

import (
	"fmt"

	app "github.com/appdynamics/cluster-agent/appd"

	"strconv"

	m "github.com/appdynamics/cluster-agent/models"

	"k8s.io/api/core/v1"
)

type DotNetInjector struct {
	Bag            *m.AppDBag
	AppdController *app.ControllerClient
}

//on new deployment add env vars to the deployment
// add init container with the .net agent
// mount agent folder to /opt/appdynamics/dotnet of the main container
//https://singularity.jira.com/wiki/spaces/~alex.ahn/pages/633014513/.NET+Agent+for+Linux

func NewDotNetInjector(bag *m.AppDBag, appdController *app.ControllerClient) DotNetInjector {
	return DotNetInjector{Bag: bag, AppdController: appdController}
}

func (dni *DotNetInjector) AddEnvVars(container *v1.Container, agentRequest *m.AgentRequest) {
	if container == nil {
		return
	}

	fmt.Printf("Adding env vars to the spec of dotnet container %s\n", container.Name)

	if container.Env == nil || len(container.Env) == 1 {
		container.Env = []v1.EnvVar{}
	}
	mountPath := GetVolumePath(dni.Bag, agentRequest)
	//key reference
	keyRef := v1.SecretKeySelector{Key: APPD_SECRET_KEY_NAME, LocalObjectReference: v1.LocalObjectReference{
		Name: APPD_SECRET_NAME}}
	envVarKey := v1.EnvVar{Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY", ValueFrom: &v1.EnvVarSource{SecretKeyRef: &keyRef}}
	envVarProfiler := v1.EnvVar{Name: "CORECLR_PROFILER", Value: "{57e1aa68-2229-41aa-9931-a6e93bbc64d8}"}
	envVarProfilerEnable := v1.EnvVar{Name: "CORECLR_ENABLE_PROFILING", Value: "1"}
	envVarProfilerPath := v1.EnvVar{Name: "CORECLR_PROFILER_PATH", Value: fmt.Sprintf("%s/libappdprofiler.so", mountPath)}
	envVarControllerHost := v1.EnvVar{Name: "APPDYNAMICS_CONTROLLER_HOST_NAME", Value: dni.Bag.ControllerUrl}
	envVarControllerPort := v1.EnvVar{Name: "APPDYNAMICS_CONTROLLER_PORT", Value: strconv.Itoa(int(dni.Bag.ControllerPort))}
	envVarControllerSSL := v1.EnvVar{Name: "APPDYNAMICS_CONTROLLER_SSL_ENABLED", Value: strconv.FormatBool(dni.Bag.SSLEnabled)}
	envVarAccountName := v1.EnvVar{Name: "APPDYNAMICS_AGENT_ACCOUNT_NAME", Value: dni.Bag.Account}
	envVarAppName := v1.EnvVar{Name: "APPDYNAMICS_AGENT_APPLICATION_NAME", Value: agentRequest.AppName}
	envVarTierName := v1.EnvVar{Name: "APPDYNAMICS_AGENT_TIER_NAME", Value: agentRequest.TierName}
	envVarNodeReuse := v1.EnvVar{Name: "APPDYNAMICS_AGENT_REUSE_NODE_NAME", Value: "true"}
	envVarNodePrefix := v1.EnvVar{Name: "APPDYNAMICS_AGENT_REUSE_NODE_NAME_PREFIX", Value: agentRequest.TierName}

	container.Env = append(container.Env, envVarKey)
	container.Env = append(container.Env, envVarProfiler)
	container.Env = append(container.Env, envVarProfilerEnable)
	container.Env = append(container.Env, envVarProfilerPath)
	container.Env = append(container.Env, envVarControllerHost)
	container.Env = append(container.Env, envVarControllerPort)
	container.Env = append(container.Env, envVarControllerSSL)
	container.Env = append(container.Env, envVarAccountName)
	container.Env = append(container.Env, envVarAppName)
	container.Env = append(container.Env, envVarTierName)
	container.Env = append(container.Env, envVarNodeReuse)
	container.Env = append(container.Env, envVarNodePrefix)

	if agentRequest.BiQRequested() {
		if agentRequest.BiQ == string(m.Sidecar) {
			envVarBiqHost := v1.EnvVar{Name: "APPDYNAMICS_ANALYTICS_HOST_NAME", Value: "localhost"}
			envVarBiqPort := v1.EnvVar{Name: "APPDYNAMICS_ANALYTICS_PORT", Value: "9090"}
			envVarBiqSSL := v1.EnvVar{Name: "APPDYNAMICS_ANALYTICS_SSL_ENABLED", Value: "false"}
			container.Env = append(container.Env, envVarBiqHost)
			container.Env = append(container.Env, envVarBiqPort)
			container.Env = append(container.Env, envVarBiqSSL)
		} else {
			envVarBiqHost := v1.EnvVar{Name: "APPDYNAMICS_ANALYTICS_HOST_NAME", Value: dni.Bag.RemoteBiqHost}
			envVarBiqPort := v1.EnvVar{Name: "APPDYNAMICS_ANALYTICS_PORT", Value: fmt.Sprintf("%d", dni.Bag.RemoteBiqPort)}
			ssl := "false"
			if dni.Bag.RemoteBiqProtocol == "https" {
				ssl = "true"
			}
			envVarBiqSSL := v1.EnvVar{Name: "APPDYNAMICS_ANALYTICS_SSL_ENABLED", Value: ssl}
			container.Env = append(container.Env, envVarBiqHost)
			container.Env = append(container.Env, envVarBiqPort)
			container.Env = append(container.Env, envVarBiqSSL)
		}

	}

}
