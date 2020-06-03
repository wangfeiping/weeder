package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	VERSION = "v0.0.1"
	API     = "https://dev-apis.qianbao.com/basicservice/v1/intranet/filer"
)

var rootCmd = &cobra.Command{
	Use:   "uploader",
	Short: "uploader - 命令行文件批量上传工具，可上传整个目录（包括子目录）",
	Long:  `uploader - 命令行文件批量上传工具，可上传整个目录（包括子目录）。`,
	Run: func(cmd *cobra.Command, args []string) {
		dir := cmd.Flag("dir").Value.String()
		prefix := cmd.Flag("prefix").Value.String()
		api := cmd.Flag("api").Value.String()
		if dir == "" && prefix == "" {
			cmd.Help()
		} else {
			err := upload(mBinPath, dir, prefix, api)
			if err != nil {
				fmt.Println("error: ", err.Error())
			}
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "uploader 版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("uploader", VERSION)
	},
}

func init() {
	rootCmd.Flags().StringP("dir", "d", "", `上传文件读取路径`)
	viper.BindPFlag("dir", rootCmd.Flags().Lookup("dir"))
	rootCmd.Flags().StringP("prefix", "p", "", `上传路径前缀`)
	viper.BindPFlag("prefix", rootCmd.Flags().Lookup("prefix"))
	rootCmd.Flags().StringP("api", "a", API, `上传api,`)
	viper.BindPFlag("api", rootCmd.Flags().Lookup("api"))
	rootCmd.AddCommand(versionCmd)
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
	if strings.HasSuffix(execPath, "/bin/") {
		execPath = execPath[0 : len(execPath)-4]
	}
	return
}

var mBinPath string

func main() {
	mBinPath = getExecPath()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
