package config

// DeploymentRoutingEnabled delegates deployment-routing policy ownership to Flux runtime.
func DeploymentRoutingEnabled(s Settings) bool {
	engine, err := newFluxEngine()
	return err == nil && engine.DeploymentRoutingEnabled(s.DeploymentRouting)
}
