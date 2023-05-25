package main

import (
	"fmt"
	"log"
	"unsafe"

	// "github.com/chenhengjie123/gmf"
	"github.com/chenhengjie123/goav/swscale"

	"github.com/chenhengjie123/goav/avcodec"
	"github.com/chenhengjie123/goav/avutil"
)

type FrameConverter struct {
	width       int
	height      int
	bitrate     int
	codec       *avcodec.Codec
	codecCtx    *avcodec.Context
	frame       *avutil.Frame
	swsCtx      *swscale.Context
	scaledFrame *avutil.Frame
	encoder     *avcodec.Codec
	encoderCtx  *avcodec.Context
}

func NewFrameConverter(width int, height int, bitrate int) *FrameConverter {

	// Initialize the AVCodecContext and AVFrame
	avcodec.AvcodecRegisterAll()
	codec := avcodec.AvcodecFindDecoder(avcodec.CodecId(avcodec.AV_CODEC_ID_H261))
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
	frame := avutil.AvFrameAlloc()
	if frame == nil {
		fmt.Errorf("failed to allocate frame")
	}
	defer avutil.AvFrameFree(frame)

	// defer swscale.SwsFreeContext(swsCtx)
	scaledFrame := avutil.AvFrameAlloc()
	if scaledFrame == nil {
		fmt.Errorf("failed to allocate scaled frame")
	}
	defer avutil.AvFrameFree(scaledFrame)

	// Encode the scaled frame into a new packet
	encoder := avcodec.AvcodecFindEncoder(avcodec.CodecId(avcodec.AV_CODEC_ID_H264))
	if encoder == nil {
		fmt.Errorf("failed to find encoder")
	}
	encoderCtx := encoder.AvcodecAllocContext3()
	if encoderCtx == nil {
		fmt.Errorf("failed to allocate encoder context")
	}
	defer encoderCtx.AvcodecClose()

	// fixme: ffmpeg编译加上x264

	// encoderCtx.SetBitRate(int64(bitrate))
	// encoderCtx.SetWidth(width)
	// encoderCtx.SetHeight(height)
	// encoderCtx.SetTimeBase(avcodec.NewRational(1, 25))
	// encoderCtx.SetGopSize(10)
	// encoderCtx.SetMaxBFrames(2)
	// encoderCtx.SetPixFmt(avcodec.AV_PIX_FMT_YUV420P)
	if encoderCtx.AvcodecOpen2(encoder, nil) < 0 {
		fmt.Errorf("failed to open encoder")
	}

	return &FrameConverter{codec: codec, codecCtx: codecCtx, frame: frame,
		scaledFrame: scaledFrame, encoder: encoder, encoderCtx: encoderCtx}
}

// 进行帧数据转换
func (fc FrameConverter) convertFrame(frameData []byte) ([]byte, error) {
	// Decode the frame data into the AVFrame
	packet := avcodec.AvPacketAlloc()
	if packet == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	// release after usage
	defer packet.AvFreePacket()

	packet.SetData(frameData)

	response := fc.codecCtx.AvcodecSendPacket(packet)
	if response < 0 {
		return nil, fmt.Errorf("failed to send packet")
	}

	response = fc.codecCtx.AvcodecReceiveFrame((*avcodec.Frame)(unsafe.Pointer(fc.frame)))
	if response < 0 {
		return nil, fmt.Errorf("failed to receive frame")
	}

	avutil.AvSetFrame(fc.scaledFrame, fc.width, fc.height, avcodec.AV_PIX_FMT_YUV420P9)
	// scaledFrame.setWidth(width)
	// scaledFrame.SetHeight(height)
	// scaledFrame.SetFormat(int32(avcodec.AV_PIX_FMT_YUV420P9))
	if avutil.AvFrameGetBuffer(fc.scaledFrame, 32) < 0 {
		return nil, fmt.Errorf("failed to allocate buffer for scaled frame")
	}

	// Scale the frame to the desired size
	if fc.swsCtx == nil {
		privateSwsCtx := swscale.SwsGetcontext(
			fc.codecCtx.Width(), fc.codecCtx.Height(), (swscale.PixelFormat)(fc.codecCtx.PixFmt()),
			fc.width, fc.height, avcodec.AV_PIX_FMT_RGB24,
			avcodec.SWS_BILINEAR, nil, nil, nil,
		)
		if privateSwsCtx == nil {
			log.Fatalf("failed to create swscale context")
		}
		fc.swsCtx = privateSwsCtx
	}

	if swscale.SwsScale2(
		fc.swsCtx, avutil.Data(fc.frame), avutil.Linesize(fc.frame),
		0, fc.codecCtx.Height(),
		avutil.Data(fc.scaledFrame), avutil.Linesize(fc.scaledFrame),
	) < 0 {
		return nil, fmt.Errorf("failed to scale frame")
	}

	encodedPacket := avcodec.AvPacketAlloc()
	if encodedPacket == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	defer encodedPacket.AvFreePacket()

	if fc.encoderCtx.AvcodecSendFrame((*avcodec.Frame)(unsafe.Pointer(fc.scaledFrame))) < 0 {
		return nil, fmt.Errorf("failed to send frame to encoder")
	}
	if fc.encoderCtx.AvcodecReceivePacket(packet) < 0 {
		return nil, fmt.Errorf("failed to receive packet from encoder")
	}

	// Return the encoded packet data
	return (*[1 << 30]byte)(unsafe.Pointer(encodedPacket.Data()))[:encodedPacket.Size():encodedPacket.Size()], nil
}
