package main

import (
	"github.com/chenhengjie123/quicktime_video_hack/screencapture"
	log "github.com/sirupsen/logrus"
	"sync"
	// "time"
)

const (
	PPS = 8
	SPS = 7
	SEI = 6
	IDR = 5
)

type ReceiverHub struct {
	udid           string
	streaming      bool
	closed         bool
	send           chan []byte
	clients        map[*Client]*ClientReceiveStatus
	stopReading    chan interface{}
	stopSignal     chan interface{}
	timeoutChannel chan bool
	writer         *NaluWriter
	sei            []byte
	pps            []byte
	sps            []byte
	mutex          *sync.Mutex
}

type ClientReceiveStatus struct {
	gotPPS    bool
	gotSPS    bool
	gotSEI    bool
	gotIFrame bool
}

func NewReceiver(udid string) *ReceiverHub {
	return &ReceiverHub{
		clients:        make(map[*Client]*ClientReceiveStatus),
		send:           make(chan []byte),
		stopSignal:     make(chan interface{}),
		timeoutChannel: make(chan bool),
		udid:           udid,
		mutex:          &sync.Mutex{},
	}
}

func (r *ReceiverHub) storeNalUnit(dst *[]byte, b *[]byte) {
	*dst = make([]byte, len(*b))
	copy(*dst, *b)
}

func (r *ReceiverHub) AddClient(c *Client) {
	_, ok := r.clients[c]
	if ok {
		log.Warn("ReceiverHub. ", "Client already added")
		return
	}
	status := &ClientReceiveStatus{}
	r.clients[c] = status
	log.Debugf("[%s]Add client: %v", r.udid, c)
	
	// ReceiverHub 是一个设备只有一个的，因此这里steaming就是指代这个设备是否在接收数据
	if !r.streaming {
		r.streaming = true
		r.stopReading = make(chan interface{})
		go r.run()
		r.stream()
	}
	select {
	case r.timeoutChannel <- false:
		break
	default:
		break
	}
}

func (r *ReceiverHub) DelClient(c *Client) {
	r.mutex.Lock()
	delete(r.clients, c)
	r.mutex.Unlock()
	if len(r.clients) == 0 {
		go func() {
			r.timeoutChannel <- true
			// 延迟一秒再发送关闭信号（试过去掉这1秒，结果结束信号的发出会出问题）
			// time.Sleep(1 * time.Second)
			// select {
			// case r.timeoutChannel <- true:
			// 	break
			// default:
			// 	break
			// }
		}()
		go func() {
			doStop := <-r.timeoutChannel
			log.Debugf("Delete client due to receive r.timeoutChannel is true")
			if doStop {
				c.hub.deleteReceiver(r)
				r.streaming = false
				r.closed = true
				r.stopSignal <- nil
			}
		}()
	}
}

func (r *ReceiverHub) stream() {
	var udid = r.udid
	device, err := screencapture.FindIosDevice(udid)
	if err != nil {
		r.send <- toErrJSON(err, "no device found to activate")
	}

	log.Debugf("Enabling device: %v", device)
	device, err = screencapture.EnableQTConfig(device)
	if err != nil {
		log.Errorf("Error enabling QT config", err)
		r.send <- toErrJSON(err, "Error enabling QT config")
	}

	log.Debugf("device actived: ", device.DetailsMap())

	r.writer = NewNaluWriter(r)
	adapter := screencapture.UsbAdapter{}
	mp := screencapture.NewMessageProcessor(&adapter, r.stopReading, r.writer, false)
	go func() {
		err := adapter.StartReading(device, &mp, r.stopReading)
		if err != nil {
			log.Error("adapter.StartReading(device, &mp, r.stopReading): ", err)
		}
		log.Debugf("adapter.StartReading is finished")
		r.writer.Stop()
	}()
}

func (r *ReceiverHub) run() {
	for {
		select {
		case <-r.stopSignal:
			r.mutex.Lock()
			for client := range r.clients {
				delete(r.clients, client)
			}
			r.mutex.Unlock()
			r.closed = true
			r.streaming = false
			r.stopReading <- nil
			select {
			case r.timeoutChannel <- true:
			default:
			}
		case data := <-r.send:
			r.mutex.Lock()
			for client, status := range r.clients {
				if client.send == nil {
					continue
				}
				client.mutex.Lock()
				nalUnitType := data[4] & 31
				if nalUnitType == PPS {
					r.storeNalUnit(&r.pps, &data)
				} else if nalUnitType == SPS {
					r.storeNalUnit(&r.sps, &data)
				} else if nalUnitType == SEI {
					r.storeNalUnit(&r.sei, &data)
				}
				if status.gotIFrame {
					*client.send <- data
				} else {
					if !status.gotSPS && r.sps != nil {
						status.gotSPS = true
						*client.send <- r.sps
						if nalUnitType == SPS {
							client.mutex.Unlock()
							continue
						}
					}
					if !status.gotPPS && r.pps != nil {
						status.gotPPS = true
						*client.send <- r.pps
						if nalUnitType == PPS {
							client.mutex.Unlock()
							continue
						}
					}
					if !status.gotSEI && r.sei != nil {
						status.gotSEI = true
						*client.send <- r.sei
						if nalUnitType == SEI {
							client.mutex.Unlock()
							continue
						}
					}
					isIframe := nalUnitType == IDR
					if status.gotPPS && status.gotSPS && status.gotSEI && isIframe {
						status.gotIFrame = true
						*client.send <- data
					} else {
						// log.Info("Receiver. ", "skipping frame for client: ", nalUnitType)
					}
				}
				client.mutex.Unlock()
			}
			r.mutex.Unlock()
		}
	}
}
