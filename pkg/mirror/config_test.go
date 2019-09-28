package mirror

import (
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

var testConfig = `
# port that this proxy will listen on
port: 8080

log-level: debug

mirror:
  url: https://google.com 
  headers:
    - key: X-Mirror-Header
      value: example-header

primary:
  url: http://127.0.0.1:8002
  # copy all primary headers to the mirror
  do-mirror-headers: true
  do-mirror-body: true

  headers:
    - key: X-Primary-Header
      value: example-header

`

var testMirrorHeaders = http.Header(map[string][]string{
	"X-Mirror-Header": []string{"example-header"},
})

var testPrimaryHeaders = http.Header(map[string][]string{
	"X-Primary-Header": []string{"example-header"},
})

func TestConfigFile(t *testing.T) {
	cfgFile := "/tmp/go_mirror_config.yaml"

	err := ioutil.WriteFile(cfgFile, []byte(testConfig), 0644)
	defer os.Remove(cfgFile)
	assert.NoError(t, err)
	if err != nil {
		panic(err)
	}

	cfg, err := InitConfig(WithConfigFile(cfgFile))
	assert.NoError(t, err)
	assert.Equal(t, cfgFile, cfg.ConfigFile)

	headers := cfg.Mirror.HTTPHeaders()
	assert.Equal(t, testMirrorHeaders, headers)

	primaryHeaders := cfg.Primary.HTTPHeaders()
	assert.Equal(t, testPrimaryHeaders, primaryHeaders)
}

func TestConfigFlags(t *testing.T) {

	viper.Set("primary-headers", "X-Primary-Header=example-header")
	viper.Set("mirror-headers", "X-Mirror-Header=example-header")

	cfg, err := InitConfig()
	assert.NoError(t, err)

	assert.Empty(t, viper.ConfigFileUsed())
	headers := cfg.Mirror.HTTPHeaders()
	assert.Equal(t, testMirrorHeaders, headers)

	primaryHeaders := cfg.Primary.HTTPHeaders()
	assert.Equal(t, testPrimaryHeaders, primaryHeaders)
}
