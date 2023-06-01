package main

import (
	"encoding/binary"

	"github.com/chenhengjie123/quicktime_video_hack/screencapture/coremedia"
	log "github.com/sirupsen/logrus"
)

var startCode = []byte{00, 00, 00, 01}
var tmpIndex = 0
var totalFrameData = []byte{}
var sps, pps, sei []byte

type NaluWriter struct {
	receiver             *ReceiverHub
	frameConverter       *FrameConverter
	videoDimensionWidth  int
	videoDimensionHeight int
}

func NewNaluWriter(client *ReceiverHub) *NaluWriter {
	frameConverter := NewFrameConverter(1280, 720, 1000000)
	return &NaluWriter{receiver: client, frameConverter: frameConverter}
}

func (nw NaluWriter) consumeVideo(buf coremedia.CMSampleBuffer) error {

	// 转码后I帧自带SPS和PPS，不需要单独发送

	ppsAndSpsData := []byte{}

	if buf.HasFormatDescription {
		// ppsData := append(startCode, buf.FormatDescription.PPS...)
		// spsData := append(startCode, buf.FormatDescription.SPS...)

		// ppsData := buf.FormatDescription.PPS
		// spsData := buf.FormatDescription.SPS

		// ppsAndSpsData = append(ppsAndSpsData, ppsData...)
		// ppsAndSpsData = append(ppsAndSpsData, spsData...)

		// SPS 和 PPS 是特殊帧，仅包含后续解析用的参数数据，不包含视频帧数据，不能用writeNalus根据quicktime格式切割帧内容
		// 因此需要单独发送。
		// 详细信息可参考：https://blog.csdn.net/huabiaochen/article/details/120321905

		// PPS 帧，直接发送
		err := nw.writeNalu(buf.FormatDescription.PPS)
		if err != nil {
			return err
		}
		// SPS 帧，直接发送
		err = nw.writeNalu(buf.FormatDescription.SPS)
		if err != nil {
			return err
		}

		// 获取原始视频宽高
		nw.videoDimensionWidth = int(buf.FormatDescription.VideoDimensionWidth)
		nw.videoDimensionHeight = int(buf.FormatDescription.VideoDimensionHeight)
	}

	if !buf.HasSampleData() {
		return nil
	}
	// 从这里开始，发送具体的视频帧数据。因为可能不止一帧数据，所以通过 writeNalus 方法识别长度，进行切割再一帧一帧发送
	return nw.writeNalus(buf.SampleData, ppsAndSpsData)
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

func (nw NaluWriter) writeNalus(bytes []byte, spsAndPpsData []byte) error {
	slice := bytes
	// isFirstFrame := true
	for len(slice) > 0 {
		// 这里是在根据大端序（quicktime用的就是大端序），把数据的前4位转换为10进制数字，这个数字表示了后面视频帧的有效数据长度
		// 参考文档：https://www.cnblogs.com/-wenli/p/12323809.html
		length := binary.BigEndian.Uint32(slice)

		// frameData := append(startCode, slice[4:length+4]...)
		frameData := slice[4 : length+4]

		// // 首帧数据，加上 SPS 和 PPS 数据
		// if isFirstFrame && len(spsAndPpsData) > 0 {
		// 	frameData = append(spsAndPpsData, frameData...)
		// 	isFirstFrame = false
		// }

		err := nw.writeNalu(frameData)
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
		log.Debug("Send bytes with length: ", len(bytes))
		// 发送具体的 nalu 单元数据给 receiver 的 send 通道。receiver 会再把这些数据发送给对应的 websocket client
		// nw.receiver.send <- append(startCode, bytes...)
		frameData := append(startCode, bytes...)

		// quicktime 传输帧数据的时候，pps、sps、sei、idr 帧都是单独的帧，不包含在一起。
		// 因此遇到首个idr帧的时候，需要在它前面补上 pps、sps、sei 帧数据，才能形成完整首帧数据，进行转码
		nalUnitType := frameData[4] & 31
		if nalUnitType == PPS {
			pps = frameData
			return nil
		} else if nalUnitType == SPS {
			sps = frameData
			return nil
		} else if nalUnitType == SEI {
			sei = frameData
			return nil
		} else if nalUnitType == IDR {
			// 统一加上 pps、sps、sei 帧数据
			firstFrameData := append(sps, pps...)
			firstFrameData = append(firstFrameData, sei...)
			firstFrameData = append(firstFrameData, frameData...)
			frameData = firstFrameData
		} else {
			// 其它帧，直接发送
			frameData = frameData
		}

		// 进行转码
		nw.frameConverter.originWidth = nw.videoDimensionWidth
		nw.frameConverter.originHeight = nw.videoDimensionHeight
		convertedData, err := nw.frameConverter.convertFrame(frameData)
		if err != nil {
			log.Error("Failed to convert frame: ", err)
		}

		// 发送转码后的帧数据
		nw.receiver.send <- convertedData

		// totalFrameData = append(totalFrameData, frameData...)
		// log.Info("合并帧数据长度：", len(totalFrameData), tmpIndex)

		// if tmpIndex == 120 {
		// 	filename := fmt.Sprintf("./tmp.h264")
		// 	log.Info("写入文件：", filename)
		// 	err := ioutil.WriteFile(filename, totalFrameData, 0644)
		// 	if err != nil {
		// 		log.Error("写入文件出错：", err)
		// 	}
		// }

		// tmpIndex++

	}
	return nil
}

func (nw NaluWriter) Stop() {

}
