package utils

import "github.com/kitex-contrib/config-etcd/etcd"

// Option is used to custom Options.
type Option interface {
	Apply(*Options)
}

// Options is used to initialize the nacos config suit or option.
type Options struct {
	EtcdCustomFunctions []etcd.CustomFunction
}
