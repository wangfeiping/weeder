/**
 * SeaweedFs 访问代理，定制与封装restful api 接口
 */
package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wangfeiping/log"

	logger "github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"
)

const (
	FlagHome   = "home"
	FlagConfig = "config"
)

func main() {
	defer log.Flush()

	cobra.EnableCommandSorting = false

	rootCmd := &cobra.Command{
		Use:   "weeder",
		Short: ShortDescription,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			viper.BindPFlags(cmd.Flags())
			log.Config(log.RollingFileConfig())
			log.Infof("starting at %s", getExecPath())
			return nil
		},
	}
	rootCmd.PersistentFlags().String(log.FlagLogFile, "./logs/weeder.log", "log file path")
	rootCmd.PersistentFlags().Int(log.FlagSize, 10, "log size(MB)")

	// Construct Root Command
	rootCmd.AddCommand(
		cmdStart(),
		cmdVersion())

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Failed executing command: %s, exiting...\n", err)
		os.Exit(1)
	}
}

// cmdStart command for start the proxy
func cmdStart() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("init")
			execPath := getExecPath()
			fmt.Println("path: ", execPath)

			configOpt := viper.GetString(FlagConfig)
			fmt.Println("config: ", configOpt)

			//读取配置文件并解析
			config, err := util.LoadConfig(configOpt)
			if "" == config.LogHost {
				logger.InitLogHost(getLocalIP())
			} else {
				logger.InitLogHost(config.LogHost)
			}
			logger.DebugS("main", "config: ", configOpt)
			if err != nil {
				logger.ErrorS("main", "{\"detail\":\"load config error: ", err, "\"}")
				return err
			}
			if !serv(config) {

			}
			return nil
		},
	}

	cmd.Flags().StringP(FlagConfig, "c", "./weeder.conf", "config file path")

	return cmd
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
	}
	return ""
}

func getExecPath() (execPath string) {
	//解析执行程序所在路径
	file, _ := exec.LookPath(os.Args[0])
	execFile := filepath.Base(file)
	execPath, _ = filepath.Abs(file)
	if len(execPath) > 1 {
		rs := []rune(execPath)
		execPath = string(rs[0:(len(execPath) - len(execFile))])
	}
	return
}
