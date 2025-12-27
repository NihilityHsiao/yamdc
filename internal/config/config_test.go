package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tailscale/hujson"
)

const (
	testData = `
{
	/* this is a test comment */
	"a": 1,
	"b": 3.14, // hello?
	"c": true,
	//also comment here
	"d": ["a", "b"], //asdasdsadasd
}
	`
)

type testSt struct {
	A int      `json:"a"`
	B float64  `json:"b"`
	C bool     `json:"c"`
	D []string `json:"d"`
}

func TestJsonWithComments(t *testing.T) {
	st := &testSt{}
	data, err := hujson.Standardize([]byte(testData))
	assert.NoError(t, err)
	err = json.Unmarshal(data, st)
	assert.NoError(t, err)
	t.Logf("%+v", *st)
	assert.Equal(t, 1, st.A)
	assert.Equal(t, 3.14, st.B)
	assert.Equal(t, true, st.C)
}

// TestForcedPluginConfig 测试强制插件配置的解析
func TestForcedPluginConfig(t *testing.T) {
	testConfig := `{
		"scan_dir": "/tmp/scan",
		"save_dir": "/tmp/save",
		"data_dir": "/tmp/data",
		"switch_config": {
			"forced_plugin": "javbus"
		}
	}`

	data, err := hujson.Standardize([]byte(testConfig))
	assert.NoError(t, err)

	c := defaultConfig()
	err = json.Unmarshal(data, c)
	assert.NoError(t, err)
	assert.Equal(t, "javbus", c.SwitchConfig.ForcedPlugin)
}

// TestForcedPluginConfig_Empty 测试空的强制插件配置
func TestForcedPluginConfig_Empty(t *testing.T) {
	testConfig := `{
		"scan_dir": "/tmp/scan",
		"save_dir": "/tmp/save",
		"data_dir": "/tmp/data",
		"switch_config": {
			"enable_search_meta_cache": true
		}
	}`

	data, err := hujson.Standardize([]byte(testConfig))
	assert.NoError(t, err)

	c := defaultConfig()
	err = json.Unmarshal(data, c)
	assert.NoError(t, err)
	assert.Equal(t, "", c.SwitchConfig.ForcedPlugin)
}

// TestForcedURLConfig 测试强制URL配置的解析
func TestForcedURLConfig(t *testing.T) {
	testConfig := `{
		"scan_dir": "/tmp/scan",
		"save_dir": "/tmp/save",
		"data_dir": "/tmp/data",
		"switch_config": {
			"forced_plugin": "manyvids",
			"forced_url": "https://www.manyvids.com/Video/12345/xxx"
		}
	}`

	data, err := hujson.Standardize([]byte(testConfig))
	assert.NoError(t, err)

	c := defaultConfig()
	err = json.Unmarshal(data, c)
	assert.NoError(t, err)
	assert.Equal(t, "manyvids", c.SwitchConfig.ForcedPlugin)
	assert.Equal(t, "https://www.manyvids.com/Video/12345/xxx", c.SwitchConfig.ForcedURL)
}

// TestForcedURLConfig_OnlyPlugin 测试只有强制插件没有URL
func TestForcedURLConfig_OnlyPlugin(t *testing.T) {
	testConfig := `{
		"scan_dir": "/tmp/scan",
		"save_dir": "/tmp/save",
		"data_dir": "/tmp/data",
		"switch_config": {
			"forced_plugin": "javbus"
		}
	}`

	data, err := hujson.Standardize([]byte(testConfig))
	assert.NoError(t, err)

	c := defaultConfig()
	err = json.Unmarshal(data, c)
	assert.NoError(t, err)
	assert.Equal(t, "javbus", c.SwitchConfig.ForcedPlugin)
	assert.Equal(t, "", c.SwitchConfig.ForcedURL)
}
