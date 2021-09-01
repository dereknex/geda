package bootstrap

import (
	"context"
	"fmt"
	"net"
	"path/filepath"

	"kubeease.com/kubeease/geda/pkg/k3s"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/server"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

func fillServerConfig(cfg *server.Config) error {
	cfg.Rootless = true
	cfg.DisableServiceLB = true
	cfg.DisableAgent = false

	// ControlConfig
	// ClusterIPRanges
	_, ipNet, err := net.ParseCIDR(DefaultCIDR)
	if err != nil {
		return errors.Wrapf(err, "invalid cluster-cidr %s", DefaultCIDR)
	}
	cfg.ControlConfig.ClusterIPRanges = append(cfg.ControlConfig.ClusterIPRanges, ipNet)

	cfg.ControlConfig.DataDir, err = datadir.LocalHome(DefaultDataDir, true)
	if err != nil {
		return errors.Wrapf(err, "invaild data dir %s", DefaultDataDir)
	}

	cfg.ControlConfig.ServiceNodePortRange, err = utilnet.ParsePortRange(DefaultNodePortRange)
	if err != nil {
		return errors.Wrapf(err, "parse node port range failed: %s", DefaultNodePortRange)
	}
	cfg.ControlConfig.HTTPSPort = DefaultHTTPSPort
	cfg.ControlConfig.SupervisorPort = DefaultHTTPSPort
	cfg.ControlConfig.APIServerPort = DefaultHTTPSPort + 1
	cfg.ControlConfig.APIServerBindAddress = "0.0.0.0"

	cfg.ControlConfig.DisableCCM = true
	cfg.ControlConfig.DisableETCD = false
	cfg.ControlConfig.DisableKubeProxy = true
	cfg.ControlConfig.DisableNPC = true

	return nil
}

func fillAgentConfig(cfg *cmds.Agent, serverCfg *server.Config) error {
	cfg.Debug = false
	cfg.DataDir = filepath.Dir(serverCfg.ControlConfig.DataDir)
	cfg.ServerURL = fmt.Sprintf("https://%s:%d", serverCfg.ControlConfig.BindAddress, serverCfg.ControlConfig.SupervisorPort)
	cfg.Token = serverCfg.ControlConfig.Runtime.AgentToken
	cfg.DisableLoadBalancer = !serverCfg.ControlConfig.DisableAPIServer
	cfg.ETCDAgent = false
	cfg.ClusterReset = serverCfg.ControlConfig.ClusterReset
	cfg.Rootless = serverCfg.Rootless
	if cfg.Rootless {
		// let agent specify Rootless kubelet flags, but not unshare twice
		cfg.RootlessAlreadyUnshared = true
	}
	return nil
}

func StartCluster(ctx context.Context) (*server.Config, error) {
	serverCfg := server.Config{}
	err := fillServerConfig(&serverCfg)
	if err != nil {
		return nil, err
	}
	// if err := server.StartServer(ctx, &serverCfg); err != nil {
	// 	return nil, err
	// }
	if err := k3s.StartServer(ctx, &serverCfg); err != nil {
		return nil, err
	}
	// run agent
	//agentCfg := cmds.Agent{}
	//err = fillAgentConfig(&agentCfg, &serverCfg)
	//if err != nil {
	//	return nil, err
	//}
	//err = agent.Run(ctx, agentCfg)
	//if err != nil {
	//	return nil, errors.Wrap(err, "run k3s agent failed")
	//}
	return &serverCfg, nil
}
