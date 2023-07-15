package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	ncdefaults "github.com/containerd/nerdctl/pkg/defaults"
	"github.com/coreos/go-systemd/activation"
	"github.com/coreos/go-systemd/daemon"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Router struct{}

var (
	debug  bool
	addr   string
	socket string
)

// func NoArgs(cmd *cobra.Command, args []string) error {
// 	if len(args) == 0 {
// 		return nil
// 	}

// 	if cmd.HasSubCommands() {
// 		return errors.Errorf("\n" + strings.TrimRight(cmd.UsageString(), "\n"))
// 	}

// 	return errors.Errorf(
// 		"\"%s\" accepts no argument(s).\nSee '%s --help'.\n\nUsage:  %s\n\n%s",
// 		cmd.CommandPath(),
// 		cmd.CommandPath(),
// 		cmd.UseLine(),
// 		cmd.Short,
// 	)
// }

func initLogging(_, stderr io.Writer) {
	logrus.SetOutput(stderr)
}

// Add daemon
func SetupRootCommand(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
	rootCmd.PersistentFlags().StringVar(&addr, "addr", "", "listening address")
	rootCmd.PersistentFlags().StringVar(&socket, "socket", "nerdctl.sock", "location of socket file")
}

func runDaemon() error {
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// router := gin.Default()
	router := newRouter()
	// router := setupRouter()

	// deprecated parameter
	if addr == "" && socket != "" {
		addr = "unix://" + socket
	}
	addrSlice := strings.SplitN(addr, "://", 2)
	if len(addrSlice) < 2 {
		return fmt.Errorf("did you mean unix://%s", addr)
	}
	proto := addrSlice[0]
	listenAddr := addrSlice[1]
	switch proto {
	case "tcp":
		return router.Run(listenAddr)
	case "fd":
		_, err := daemon.SdNotify(false, daemon.SdNotifyReady)
		if err != nil {
			return err
		}
		files := activation.Files(true)
		return router.RunFd(int(files[0].Fd()))
	case "unix":
		socket := listenAddr
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigs
			// http.Serve never returns, if successful
			os.Remove(socket)
			os.Exit(0)
		}()
		return router.RunUnix(socket)
	default:
		return fmt.Errorf("addr %s not supported", addr)
	}
}

func newDaemonCommand() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:           "nerdctld [OPTIONS]",
		Short:         "A nerdctl daemon to use nerdctl.",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Args:          NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon()
		},
		DisableFlagsInUseLine: true,
	}
	SetupRootCommand(cmd)

	return cmd, nil
}

func newRouter() *gin.Engine {
	// r := &Router{}
	g := gin.Default()
	err := g.SetTrustedProxies(nil)
	if err != nil {
		log.Print(err)
	}

	// // IMAGE ROUTERS
	// g.GET("/:ver/images/json", r.getImageJSON)
	// g.GET("/:ver/images/:name/json", r.getImagesByName)
	// g.GET("/:ver/images/get", r.getImagesGet)
	// g.GET("/:ver/images/:name/get", r.getImagesGet)
	// g.GET("/:ver/images/:name/history", r.getImagesHistory)
	// g.POST("/:ver/images/create", r.postImagesCreate)
	// g.POST("/:ver/images/:name/push", r.postImagesPush)
	// g.POST("/:ver/images/:name/tag", r.postImagesTag)
	// g.POST("/:ver/images/load", r.postImagesLoad)
	// g.DELETE("/:ver/images/:name", r.deleteImages)

	// ////////////////// CONTAINER ROUTERS /////////////
	// g.GET("/:ver/containers/json", r.getContainerJSON)
	// g.GET("/:ver/containers/:name/json", r.getContainersByName)
	// g.GET("/:ver/containers/:name/top", r.getContainersTop)
	// g.GET("/:ver/containers/:name/logs", r.getContainersLogs)
	g.POST("/:ver/containers/create", postContainersCreate)
	// g.POST("/:ver/containers/:id/attach", r.postContainersAttach)
	// g.POST("/:ver/containers/:id/kill", r.postContainersKill)
	// g.POST("/:ver/containers/:id/start", r.postContainersStart)
	// g.POST("/:ver/containers/:id/stop", r.postContainersStop)
	// g.POST("/:ver/containers/:id/restart", r.postContainersRestart)
	// g.POST("/:ver/containers/prune", r.postContainersPrune)
	// g.POST("/:ver/containers/:id/wait", r.postContainersWait)
	// g.POST("/:ver/containers/:id/resize", r.postContainersResize)

	// g.DELETE("/:ver/containers/:id", r.deleteContainers)

	// ///////////////////NETWORK//////////////////////
	// g.POST("/:ver/networks/create", r.postNetworksCreate)
	// g.GET("/:ver/networks", r.getNetworksList)
	// g.GET("/:ver/networks/", r.getNetworksList)
	// g.DELETE("/:ver/networks/:id", r.deleteNetwork)

	// //////////////// SYSTEM ROUTERS  //////////////
	// g.GET("/:ver/version", r.getVersion)
	g.GET("/_ping", getPingHandler)
	g.HEAD("/_ping", headPingHandler)
	// g.GET("/:ver/info", r.getInfo)

	return g
}

const (
	CurrentAPIVersion = "1.43"
	MinimumAPIVersion = "1.23"
)

func headPingHandler(c *gin.Context) {
	c.Writer.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Writer.Header().Add("Pragma", "no-cache")
	c.Writer.Header().Set("API-Version", CurrentAPIVersion)
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.Header().Set("Content-Length", "0")
	c.Status(http.StatusOK)
}

func getPingHandler(c *gin.Context) {

	c.Writer.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Writer.Header().Add("Pragma", "no-cache")
	c.Writer.Header().Set("API-Version", MinimumAPIVersion)
	c.String(http.StatusOK, "OK")
}

func postContainersCreate(g *gin.Context) {
	fmt.Println("Test to create commandline")
	args := []string{"ubuntu:22.04", "-i", "-t", "--name", "test"}
	var err error

	rootCmd := &cobra.Command{
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true, // required for global short hands like -a, -H, -n
	}
	rootCmd.SetUsageFunc(usage)
	aliasToBeInherited, err := initRootCmdFlags(rootCmd, ncdefaults.NerdctlTOML())
	if err != nil {
		return
	}
	processRootCmdFlags(rootCmd)
	rootCmd.SetContext(context.Background())

	rootCmd.SetArgs(args)
	rootCmd.Flags().SetInterspersed(false)
	setCreateFlags(rootCmd)
	rootCmd.InheritedFlags().AddFlagSet(aliasToBeInherited)
	err = createAction(rootCmd, args)
	if err != nil {
		logrus.Errorf(err.Error())
	}
}
