package k3s

import (
	"context"
	net2 "net"
	"os"
	"time"

	"github.com/rancher/k3s/pkg/daemons/control"

	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"

	"go.uber.org/zap"

	"github.com/rancher/k3s/pkg/clientaccess"

	"github.com/rancher/k3s/pkg/daemons/config"

	"github.com/rancher/k3s/pkg/apiaddresses"
	"github.com/rancher/k3s/pkg/node"
	"github.com/rancher/k3s/pkg/rootlessports"
	"github.com/rancher/wrangler/pkg/leader"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/nodepassword"
	"github.com/rancher/k3s/pkg/server"
	log "github.com/sirupsen/logrus"
	"kubeease.com/kubeease/geda/pkg/version"
)

const (
	MasterRoleLabelKey       = "node-role.kubernetes.io/master"
	ControlPlaneRoleLabelKey = "node-role.kubernetes.io/control-plane"
)

func StartServer(ctx context.Context, config *server.Config) error {
	if err := control.Server(ctx, &config.ControlConfig); err != nil {
		return errors.Wrap(err, "starting kubernetes")
	}
	go startOnAPIServerReady(ctx, config)
	ip := net2.ParseIP(config.ControlConfig.BindAddress)
	if ip == nil {
		hostIP, err := net.ChooseHostInterface()
		if err == nil {
			ip = hostIP
		} else {
			ip = net2.ParseIP("127.0.0.1")
		}
	}

	if err := printTokens(ip.String(), &config.ControlConfig); err != nil {
		return err
	}
	return nil
}

func startOnAPIServerReady(ctx context.Context, config *server.Config) {
	select {
	case <-ctx.Done():
		return
	case <-config.ControlConfig.Runtime.APIServerReady:
		if err := runControllers(ctx, config); err != nil {
			log.Fatalf("failed to start controllers: %v", err)
		}
	}
}

func runControllers(ctx context.Context, config *server.Config) error {
	controlConfig := &config.ControlConfig

	sc, err := server.NewContext(ctx, controlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}

	// run migration before we set controlConfig.Runtime.Core
	if err := nodepassword.MigrateFile(
		sc.Core.Core().V1().Secret(),
		sc.Core.Core().V1().Node(),
		controlConfig.Runtime.NodePasswdFile); err != nil {
		log.Error("error migrating node-password file", zap.Error(err))
	}
	controlConfig.Runtime.Core = sc.Core

	if controlConfig.Runtime.ClusterControllerStart != nil {
		if err := controlConfig.Runtime.ClusterControllerStart(ctx); err != nil {
			return errors.Wrap(err, "starting cluster controllers")
		}
	}

	for _, controller := range config.Controllers {
		if err := controller(ctx, sc); err != nil {
			return errors.Wrap(err, "controller")
		}
	}

	if err := sc.Start(ctx); err != nil {
		return err
	}

	start := func(ctx context.Context) {
		if err := coreControllers(ctx, sc, config); err != nil {
			panic(err)
		}
		for _, controller := range config.LeaderControllers {
			if err := controller(ctx, sc); err != nil {
				panic(errors.Wrap(err, "leader controller"))
			}
		}
		if err := sc.Start(ctx); err != nil {
			panic(err)
		}
	}

	go setControlPlaneRoleLabel(ctx, sc.Core.Core().V1().Node(), config)

	go setClusterDNSConfig(ctx, config, sc.Core.Core().V1().ConfigMap())

	if controlConfig.NoLeaderElect {
		go func() {
			start(ctx)
			<-ctx.Done()
			log.Fatal("controllers exited")
		}()
	} else {
		go leader.RunOrDie(ctx, "", version.Program, sc.K8s, start)
	}

	return nil
}

func setControlPlaneRoleLabel(ctx context.Context, nodes v1.NodeClient, config *server.Config) {
	if config.DisableAgent || config.ControlConfig.DisableAPIServer {
		return
	}
	for {
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			log.Info("Waiting for control-plane node agent startup")
			time.Sleep(1 * time.Second)
			continue
		}
		n, err := nodes.Get(nodeName, metav1.GetOptions{})
		if err != nil {
			log.Infof("Waiting for control-plane node %s startup: %v", nodeName, err)
			time.Sleep(1 * time.Second)
			continue
		}

		if v, ok := n.Labels[ControlPlaneRoleLabelKey]; ok && v == "true" {
			break
		}
		if n.Labels == nil {
			n.Labels = make(map[string]string)
		}
		n.Labels[ControlPlaneRoleLabelKey] = "true"
		n.Labels[MasterRoleLabelKey] = "true"

		_, err = nodes.Update(n)
		if err == nil {
			log.Infof("Control-plane role label has been set successfully on node: %s", nodeName)
			break
		}
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != nil {
				log.Infof("set control plane error: %v", err)
			}
			return
		case <-time.After(time.Second):
		}
	}
}

func coreControllers(ctx context.Context, sc *server.Context, config *server.Config) error {
	if err := node.Register(ctx,
		!config.ControlConfig.Skips["coredns"],
		sc.Core.Core().V1().Secret(),
		sc.Core.Core().V1().ConfigMap(),
		sc.Core.Core().V1().Node()); err != nil {
		return err
	}

	if err := apiaddresses.Register(ctx, config.ControlConfig.Runtime, sc.Core.Core().V1().Endpoints()); err != nil {
		return err
	}

	if config.Rootless {
		return rootlessports.Register(ctx,
			sc.Core.Core().V1().Service(),
			!config.DisableServiceLB,
			config.ControlConfig.HTTPSPort)
	}

	return nil
}

func getAgentToken(token, certs string) (string, error) {

	if len(token) == 0 {
		return "", nil
	}

	agentToken, err := clientaccess.FormatToken(token, certs)
	if err != nil {
		return "", err
	}
	return agentToken, nil

}

func printTokens(advertiseIP string, config *config.Control) error {

	if advertiseIP == "" {
		advertiseIP = "127.0.0.1"
	}
	if len(config.Runtime.ServerToken) == 0 {
		return errors.Errorf("Invaild server token %s", config.Runtime.ServerToken)
	}
	token, err := getAgentToken(config.Runtime.ServerToken, config.Runtime.ServerCA)
	if err == nil {
		log.Info("Node token is available")
	}

	//printToken(config.SupervisorPort, advertiseIP, "To join node to cluster:", "agent")
	ip := advertiseIP
	if advertiseIP == "" {
		hostIP, err := net.ChooseHostInterface()
		if err != nil {
			log.Errorf("Failed to choose interface: %v", err)
		}
		ip = hostIP.String()
	}

	log.Infof("To join node to cluster: agent %s -s https://%s:%d -t %s", version.Program, ip, config.HTTPSPort, token)

	return nil
}

func setClusterDNSConfig(ctx context.Context, controlConfig *server.Config, configMap v1.ConfigMapClient) {
	// check if configmap already exists
	_, err := configMap.Get("kube-system", "cluster-dns", metav1.GetOptions{})
	if err == nil {
		log.Infof("Cluster dns configmap already exists")
		return
	}
	clusterDNS := controlConfig.ControlConfig.ClusterDNS
	clusterDomain := controlConfig.ControlConfig.ClusterDomain
	c := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-dns",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"clusterDNS":    clusterDNS.String(),
			"clusterDomain": clusterDomain,
		},
	}
	for {
		_, err = configMap.Create(c)
		if err == nil {
			log.Infof("Cluster dns configmap has been set successfully")
			break
		}
		log.Infof("Waiting for control-plane dns startup: %v", err)

		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != nil {
				log.Errorf("set cluster dns error: %v", err)
			}
			return
		case <-time.After(time.Second):
		}
	}
}
