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
	"strings"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"

	"github.com/spf13/cobra"
)

func main() {

	rootCmd := &cobra.Command{
		Use:   "weeder",
		Short: ShortDescription,
	}

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
	return &cobra.Command{
		Use:   "start",
		Short: "start",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("init")
			execPath := getExecPath()
			fmt.Println("path: ", execPath)

			var opt, configOpt string

			if len(os.Args) > 1 {
				opt = os.Args[1]
				if strings.EqualFold(opt, "-c") && len(os.Args) > 2 {
					configOpt = os.Args[2]
				} else {
					fmt.Println("help: nohup ./weeder -c ./weeder.conf &")
					return nil
				}
			}
			if len(configOpt) < 1 {
				configOpt = execPath + "weeder.conf"
			}

			//读取配置文件并解析
			config, err := util.LoadConfig(configOpt)
			if "" == config.LogHost {
				log.InitLogHost(getLocalIP())
			} else {
				log.InitLogHost(config.LogHost)
			}
			log.DebugS("main", "config: ", configOpt)
			if err != nil {
				log.ErrorS("main", "{\"detail\":\"load config error: ", err, "\"}")
				return err
			}
			if !serv(config) {

			}
			return nil
		},
	}
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
