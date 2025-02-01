module aoanima.ru/potok

go 1.23.4

replace aoanima.ru/Logger => ../Logger

replace aoanima.ru/QErrors => ../QErrors

replace aoanima.ru/Utilites => ../Utilites

require (
	aoanima.ru/Logger v0.0.0-00010101000000-000000000000
	github.com/rodrigocfd/windigo v0.0.0-20230809154420-8faa606d9f5f
)

require (
	aoanima.ru/QErrors v0.0.0-00010101000000-000000000000 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sys v0.21.0 // indirect
)
