module aoanima.ru/potok

go 1.23.4

replace aoanima.ru/Logger => ../Logger

replace aoanima.ru/QErrors => ../QErrors

replace aoanima.ru/Utilites => ../Utilites

require (
	aoanima.ru/Logger v0.0.0-00010101000000-000000000000
	aoanima.ru/QErrors v0.0.0-00010101000000-000000000000
	github.com/rodrigocfd/windigo v0.0.0-20230809154420-8faa606d9f5f
)

require (
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sys v0.28.0 // indirect
)
