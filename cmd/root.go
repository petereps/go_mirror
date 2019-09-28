/*
Copyright © 2019 PETER EPSTEEN <peterepsteen@gmail.com>

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
	"log"
	"os"

	"github.com/petereps/go_mirror/pkg/mirror"

	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go_mirror",
	Short: "Mirror http requests to a different backend for testing or logging",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		opts := []mirror.Option{}

		cfgFile := os.Getenv("FILE")
		if cfgFile == "" {
			cfgFile, _ = cmd.PersistentFlags().GetString("file")
		}

		if cfgFile != "" {
			opts = append(opts, mirror.WithConfigFile(cfgFile))
		}

		cfg, err := mirror.InitConfig(opts...)
		if err != nil {
			panic(err)
		}

		mirrorProxy, err := mirror.New(cfg)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Serving on port %d\n", cfg.Port)
		log.Fatal(mirrorProxy.Serve(fmt.Sprintf(":%d", cfg.Port)))
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
		StringP("log-level", "l", "info", "Log level to use. Either error, warn, info, or debug")
	rootCmd.PersistentFlags().
		StringP("file", "f", "", "Specify a filepath to load config from")

	rootCmd.PersistentFlags().
		StringP("mirror-url", "m", "", "Upstream server to mirror incoming requests to (wont effect the primary servers request)")

	rootCmd.PersistentFlags().
		StringP("primary-url", "p", "", "Primary server to proxy to (responses will be returned to client)")

	rootCmd.PersistentFlags().
		IntP("port", "P", 0, "port to serve the mirror on")

	rootCmd.PersistentFlags().
		Bool("do-mirror-headers", true, "Directive to mirror all incoming headers to the mirrored server")

	rootCmd.PersistentFlags().
		Bool("do-mirror-body", true, "Directive to mirror request body to mirrored server (small performance hit)")

	rootCmd.PersistentFlags().
		StringArray("mirror-headers", []string{}, "Headers to add to the mirrored request. in the form of --mirror-headers header=value --mirror-headers header2=value2...")

	rootCmd.PersistentFlags().
		StringArray("primary-headers", []string{}, "Headers to add to the primary request. in the form of --primary-headers header=value --primary-headers header2=value2...")

	viper.BindPFlags(rootCmd.PersistentFlags())
	viper.BindPFlag("mirror.url", rootCmd.PersistentFlags().Lookup("mirror-url"))
	viper.BindPFlag("primary.url", rootCmd.PersistentFlags().Lookup("primary-url"))
	viper.BindPFlag("primary.do-mirror-headers", rootCmd.PersistentFlags().Lookup("do-mirror-headers"))
	viper.BindPFlag("primary.do-mirror-body", rootCmd.PersistentFlags().Lookup("do-mirror-body"))

}
