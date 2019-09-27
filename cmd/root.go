/*
Copyright © 2019 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/petereps/go_mirror/pkg/mirror"

	"github.com/petereps/go_mirror/pkg/config"

	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go_mirror",
	Short: "Mirror http requests to a different backend for testing or logging",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		opts := []config.Option{}

		if cfgFile := viper.GetString("file"); cfgFile != "" {
			println(cfgFile)
			opts = append(opts, config.WithConfigFile(cfgFile))
		}

		cfg, err := config.InitConfig(opts...)
		if err != nil {
			panic(err)
		}

		mirrorProxy, err := mirror.New(cfg.PrimaryServer, cfg.UpstreamMirror)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Serving on port %d\n", cfg.Port)
		mirrorProxy.Serve(fmt.Sprintf(":%d", cfg.Port))
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().
		StringP("file", "f", "", "Specify a filepath to load config from")

	rootCmd.PersistentFlags().
		StringP("mirror", "m", "", "Upstream server to mirror incoming requests to (wont effect the primary servers request)")

	rootCmd.PersistentFlags().
		StringP("server", "s", "", "Primary server to proxy to (responses will be returned to client)")

	rootCmd.PersistentFlags().
		IntP("port", "p", 8080, "port to serve the mirror on")

	viper.BindPFlags(rootCmd.PersistentFlags())
}
