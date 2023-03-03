module github.com/NetrisTV/ws-qvh

go 1.15

require (
	github.com/danielpaulus/go-ios v1.0.13
	github.com/danielpaulus/quicktime_video_hack v0.0.0-20230104211913-e021966965d4
	github.com/gorilla/websocket v1.4.1
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/objx v0.1.1 // indirect
	golang.org/x/crypto v0.0.0-20180904163835-0709b304e793 // indirect
	golang.org/x/sys v0.0.0-20210910150752-751e447fb3d0 // indirect
)

replace github.com/danielpaulus/quicktime_video_hack v0.0.0-20200913112742-92dee353674c => github.com/NetrisTV/quicktime_video_hack v0.0.0-20201026161452-fe5cb4b55736
