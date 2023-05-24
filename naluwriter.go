package main

import (
	"encoding/binary"

	"github.com/chenhengjie123/quicktime_video_hack/screencapture/coremedia"
	log "github.com/sirupsen/logrus"
)

var startCode = []byte{00, 00, 00, 01}

type NaluWriter struct {
	receiver       *ReceiverHub
	frameConverter *FrameConverter
}

func NewNaluWriter(cliend *ReceiverHub) *NaluWriter {
	return &NaluWriter{receiver: cliend, frameConverter: NewFrameConverter(1280, 720, 1000000)}
}

func (nw NaluWriter) consumeVideo(buf coremedia.CMSampleBuffer) error {

	// 转码后I帧自带SPS和PPS，不需要单独发送

	// if buf.HasFormatDescription {
	// 	// SPS 和 PPS 是特殊帧，仅包含后续解析用的参数数据，不包含视频帧数据，不能用writeNalus根据quicktime格式切割帧内容
	// 	// 因此需要单独发送。
	// 	// 详细信息可参考：https://blog.csdn.net/huabiaochen/article/details/120321905

	// 	// PPS 帧，直接发送
	// 	err := nw.writeNalu(buf.FormatDescription.PPS)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	// SPS 帧，直接发送
	// 	err = nw.writeNalu(buf.FormatDescription.SPS)
	// 	if err != nil {
	// 		return err
	// 	}

	// }
	if !buf.HasSampleData() {
		return nil
	}
	// 从这里开始，发送具体的视频帧数据。因为可能不止一帧数据，所以通过 writeNalus 方法识别长度，进行切割再一帧一帧发送
	return nw.writeNalus(buf.SampleData)
}

// 收到 quicktime 传输的数据，进行消费
func (nw NaluWriter) Consume(buf coremedia.CMSampleBuffer) error {
	if buf.MediaType == coremedia.MediaTypeSound {
		// we don't support audio for now
		//return nw.consumeAudio(buf)
		return nil
	}
	return nw.consumeVideo(buf)
}

func (nw NaluWriter) writeNalus(bytes []byte) error {
	slice := bytes
	for len(slice) > 0 {
		// 这里是在根据大端序（quicktime用的就是大端序），把数据的前4位转换为10进制数字，这个数字表示了后面视频帧的有效数据长度
		// 参考文档：https://www.cnblogs.com/-wenli/p/12323809.html
		length := binary.BigEndian.Uint32(slice)

		// fixme: 修复合并逻辑
		frameData := append(startCode, slice[4:length+4]...)
		// 前4个字节是长度，第5个开始，到长度值+4的位置，是具体的视频帧数据。所以实际发送的视频数据，只需要发送这部分即可
		convertedData, err := nw.frameConverter.convertFrame(frameData)

		if err != nil {
			log.Error("Failed to convert frame: ", err)
		}
		err = nw.writeNalu(convertedData)
		if err != nil {
			return err
		}
		// 从已写入完毕的下一个数据开始，继续这个循环，直到这次的数据全部处理完毕
		slice = slice[length+4:]
	}
	return nil
}

func (nw NaluWriter) writeNalu(bytes []byte) error {
	if nw.receiver.closed {
		return nil
	}
	if len(bytes) > 0 {
		log.Debug("Send bytes "+string(startCode)+" with length: ", len(bytes))
		// 发送具体的 nalu 单元数据给 receiver 的 send 通道。receiver 会再把这些数据发送给对应的 websocket client
		// nw.receiver.send <- append(startCode, bytes...)

		// 不添加 startCode ，直接发送
		nw.receiver.send <- bytes
	}
	return nil
}

func (nw NaluWriter) Stop() {

}
