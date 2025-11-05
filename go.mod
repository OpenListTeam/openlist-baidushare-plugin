module openlist-baidushare-plugin

go 1.23.0

require (
	github.com/OpenListTeam/openlist-wasi-plugin-driver v0.0.0-20251105181311-31306d50eadb
	github.com/pkg/errors v0.9.1
	go.bytecodealliance.org/cm v0.3.0
	resty.dev/v3 v3.0.0-beta.3
)

require github.com/OpenListTeam/go-wasi-http v0.0.0-20251015142022-5647e49e373d

replace resty.dev/v3 => github.com/OpenListTeam/resty-tinygo/v3 v3.0.0-20251013065911-8fff8b24c719
