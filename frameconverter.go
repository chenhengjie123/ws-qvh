package main

import (
	"fmt"
	"unsafe"

	// "github.com/chenhengjie123/gmf"
	"github.com/chenhengjie123/ffmpeg-go/ffcommon"
	"github.com/chenhengjie123/ffmpeg-go/libavcodec"
	"github.com/chenhengjie123/ffmpeg-go/libavutil"
	"github.com/chenhengjie123/ffmpeg-go/libswscale"
	// "github.com/giorgisio/goav/swscale"
)

type FrameConverter struct {
	width       int
	height      int
	bitrate     int
	codec       *libavcodec.AVCodec
	codecCtx    *libavcodec.AVCodecContext
	frame       *libavutil.AVFrame
	swsCtx      *libswscale.SwsContext
	scaledFrame *libavutil.AVFrame
	encoder     *libavcodec.AVCodec
	encoderCtx  *libavcodec.AVCodecContext
}

func NewFrameConverter(width int, height int, bitrate int) *FrameConverter {

	ffcommon.SetAvcodecPath("/usr/local/ffmpeg/lib/libavcodec.dylib")
	ffcommon.SetAvutilPath("/usr/local/ffmpeg/lib/libavutil.dylib")
	ffcommon.SetAvdevicePath("/usr/local/ffmpeg/lib/libavdevice.dylib")
	ffcommon.SetAvfilterPath("/usr/local/ffmpeg/lib/libavfilter.dylib")
	ffcommon.SetAvformatPath("/usr/local/ffmpeg/lib/libavformat.dylib")
	ffcommon.SetAvpostprocPath("/usr/local/ffmpeg/lib/libpostproc.dylib")
	ffcommon.SetAvswresamplePath("/usr/local/ffmpeg/lib/libswresample.dylib")
	ffcommon.SetAvswscalePath("/usr/local/ffmpeg/lib/libswscale.dylib")

	// Initialize the AVCodecContext and AVFrame
	libavcodec.AvcodecRegisterAll()
	codec := libavcodec.AvcodecFindDecoder(libavcodec.AV_CODEC_ID_H264)
	if codec == nil {
		fmt.Errorf("failed to find codec")
	}
	codecCtx := codec.AvcodecAllocContext3()
	if codecCtx == nil {
		fmt.Errorf("failed to allocate codec context")
	}
	defer codecCtx.AvcodecClose()

	if codecCtx.AvcodecOpen2(codec, nil) < 0 {
		fmt.Errorf("failed to open codec")
	}
	frame := libavutil.AvFrameAlloc()
	if frame == nil {
		fmt.Errorf("failed to allocate frame")
	}
	defer libavutil.AvFrameFree(&frame)

	// defer libswscale.SwsFreeContext(swsCtx)
	scaledFrame := libavutil.AvFrameAlloc()
	if scaledFrame == nil {
		fmt.Errorf("failed to allocate scaled frame")
	}
	defer libavutil.AvFrameFree(&scaledFrame)

	// Encode the scaled frame into a new packet
	encoder := libavcodec.AvcodecFindEncoder(libavcodec.AV_CODEC_ID_H264)
	if encoder == nil {
		fmt.Errorf("failed to find encoder")
	}
	encoderCtx := encoder.AvcodecAllocContext3()
	if encoderCtx == nil {
		fmt.Errorf("failed to allocate encoder context")
	}
	defer encoderCtx.AvcodecClose()

	// ffmpeg编译加上x264
	// FIXME: 找到下面 setParams 对应的函数
	// encoderCtx.SetEncodeParams2(width, height, (libavcodec.PixelFormat)(libavcodec.AV_PIX_FMT_YUV420P), false, 10)

	encoderCtx.Width = int32(width)
	encoderCtx.Height = int32(height)
	encoderCtx.BitRate = int64(bitrate)
	encoderCtx.GopSize = 10
	encoderCtx.MaxBFrames = 0
	encoderCtx.PixFmt = int32(libavutil.AV_PIX_FMT_YUV420P)
	encoderCtx.AvCodecSetPktTimebase(libavutil.AVRational{1, 25})

	// encoderCtx.SetBitRate(int64(bitrate))
	if encoderCtx.AvcodecOpen2(encoder, nil) < 0 {
		fmt.Errorf("failed to open encoder")
	}

	return &FrameConverter{codec: codec, codecCtx: codecCtx, frame: frame,
		scaledFrame: scaledFrame, encoder: encoder, encoderCtx: encoderCtx}
}

// PointerToBytes 函数接收一个指向内存地址的指针和一个长度参数，将指针指向的内存地址中的数据转换为字节切片并返回
func PointerToBytes(pointer unsafe.Pointer, size int) []byte {
	// 将指针转换为一个 byte 类型的指针
	byteArrayPointer := (*[1 << 30]byte)(pointer)
	// 从 byte 数组中获取对应长度的数据
	bytes := byteArrayPointer[:size]
	return bytes
}

// 进行帧数据转换
func (fc FrameConverter) convertFrame(frameData []byte) ([]byte, error) {
	// Decode the frame data into the AVFrame
	packet := libavcodec.AvPacketAlloc()
	packet.AvInitPacket()

	if packet == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	// release after usage
	defer packet.AvFreePacket()

	packet.Data = (*uint8)(unsafe.Pointer(&frameData[0]))
	packet.Size = uint32(len(frameData))
	// data := PointerToBytes(unsafe.Pointer(packet.Data()), int(packet.Size()))
	// print(data)

	response := fc.codecCtx.AvcodecSendPacket(packet)
	if response < 0 {
		return nil, fmt.Errorf("failed to send packet")
	}
	// got_picture := ffcommon.FInt(0)
	// response := fc.codecCtx.AvcodecDecodeVideo2((*libavcodec.AVFrame)(unsafe.Pointer(fc.frame)), &got_picture, packet)
	// if response < 0 {
	// 	return nil, fmt.Errorf("failed to decode video")
	// }
	// if got_picture <= 0 {
	// 	return nil, fmt.Errorf("got picture failed")
	// }

	response = fc.codecCtx.AvcodecReceiveFrame((*libavcodec.AVFrame)(unsafe.Pointer(fc.frame)))
	if response < 0 {
		return nil, fmt.Errorf("failed to receive frame")
	}

	// libavutil.frame(fc.scaledFrame, fc.width, fc.height, libavutil.AV_PIX_FMT_YUV420P9)
	// // scaledFrame.setWidth(width)
	// // scaledFrame.SetHeight(height)
	// // scaledFrame.SetFormat(int32(libavcodec.AV_PIX_FMT_YUV420P9))
	// if libavutil.AvFrameGetBuffer(fc.scaledFrame, 32) < 0 {
	// 	return nil, fmt.Errorf("failed to allocate buffer for scaled frame")
	// }

	// Scale the frame to the desired size
	// if fc.swsCtx == nil {
	// 	privateSwsCtx := libswscale.SwsGetcontext(
	// 		fc.codecCtx.Width(), fc.codecCtx.Height(), (libswscale.PixelFormat)(fc.codecCtx.PixFmt()),
	// 		fc.width, fc.height, libavcodec.AV_PIX_FMT_RGB24,
	// 		libavcodec.SWS_BILINEAR, nil, nil, nil,
	// 	)
	// 	if privateSwsCtx == nil {
	// 		log.Fatalf("failed to create swscale context")
	// 	}
	// 	fc.swsCtx = privateSwsCtx
	// }

	// if libswscale.SwsScale2(
	// 	fc.swsCtx, libavutil.Data(fc.frame), libavutil.Linesize(fc.frame),
	// 	0, fc.codecCtx.Height(),
	// 	libavutil.Data(fc.scaledFrame), libavutil.Linesize(fc.scaledFrame),
	// ) < 0 {
	// 	return nil, fmt.Errorf("failed to scale frame")
	// }

	encodedPacket := libavcodec.AvPacketAlloc()
	if encodedPacket == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	defer encodedPacket.AvFreePacket()

	if fc.encoderCtx.AvcodecSendFrame((*libavcodec.AVFrame)(unsafe.Pointer(fc.scaledFrame))) < 0 {
		return nil, fmt.Errorf("failed to send frame to encoder")
	}
	if fc.encoderCtx.AvcodecReceivePacket(packet) < 0 {
		return nil, fmt.Errorf("failed to receive packet from encoder")
	}

	// Return the encoded packet data
	return (*[1 << 30]byte)(unsafe.Pointer(encodedPacket.Data))[:encodedPacket.Size:encodedPacket.Size], nil
}
