module github.com/mattpainter701/graphlayer/platform/orchestration

go 1.21

require (
	github.com/mattpainter701/graphlayer/platform v0.0.0
	github.com/spf13/cobra v1.8.0
	golang.org/x/crypto v0.51.0
	golang.org/x/time v0.15.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/mattpainter701/graphlayer/platform => ..

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405 // indirect
)
