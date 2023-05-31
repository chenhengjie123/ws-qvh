module github.com/NetrisTV/ws-qvh

go 1.15

require (
	github.com/chenhengjie123/ffmpeg-go v0.0.0-20230531040822-56ce8b044594
	// github.com/chenhengjie123/goav v0.0.0-20230523053203-eb24917db498 // indirect
	github.com/chenhengjie123/quicktime_video_hack v0.0.0-20230401031452-1ae92f5c0848
	github.com/gorilla/websocket v1.4.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.7.1 // indirect
	golang.org/x/sys v0.6.0 // indirect
// github.com/chenhengjie123/goav v0.0.0-20220519232855-ea60062d943c
)

replace github.com/danielpaulus/quicktime_video_hack v0.0.0-20200913112742-92dee353674c => github.com/NetrisTV/quicktime_video_hack v0.0.0-20201026161452-fe5cb4b55736
