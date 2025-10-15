module openlist-baidushare-plugin

go 1.23.0

require (
	github.com/OpenListTeam/openlist-wasi-plugin-driver v0.0.0-20251015133414-5b50219c1270
	github.com/pkg/errors v0.9.1
	go.bytecodealliance.org/cm v0.3.0
	resty.dev/v3 v3.0.0-00010101000000-000000000000
)

require github.com/OpenListTeam/go-wasi-http v0.0.0-20251015142022-5647e49e373d

replace resty.dev/v3 => github.com/OpenListTeam/resty-tinygo/v3 v3.0.0-20251013065911-8fff8b24c719
