package conf

var Conf AppConf

func SetAppConf(conf AppConf) {
	Conf = conf
}

type AppConf struct {
	AppInfo struct {
		Name       string `yaml:"Name"`  //项目名
		Port       int    `yaml:"Port"`  //健康检查端口
		Level      string `yaml:"Level"` //日志等级
		TLSKey     string `yaml:"TLSKey"`
		TLSCert    string `yaml:"TLSCert"`
		TLSCaCerts string `yaml:"TLSCaCerts"`
	} `yaml:"AppInfo"`
}
