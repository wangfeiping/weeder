package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/wangfeiping/weeder/log"
	"github.com/wangfeiping/weeder/util"
)

type DataNode struct {
	Url     string `json:"Url"`
	Max     int    `json:"Max"`
	Free    int    `json:"Free"`
	Volumes int    `json:"Volumes"`
}

type Rack struct {
	Id        string     `json:"Id"`
	DataNodes []DataNode `json:"DataNodes"`
}

type DataCenter struct {
	Id    string `json:"Id"`
	Racks []Rack `json:"Racks"`
}

type DataTopo struct {
	DataCenters []DataCenter `json:"DataCenters"`
}

type SeaweedFsTopo struct {
	Topology DataTopo `json:"Topology"`
	Version  string   `json:"Version"`
}

func StartScheduleJob(config *util.WeederConfig) {
	if config.VolumeCheckDuration > 0 {
		log.DebugS("sche", "schedule job start...")
		go scheduleJob(config)
	} else {
		log.DebugS("sche", "schedule job not run.")
	}
}

func scheduleJob(config *util.WeederConfig) {
	c := time.Tick(time.Duration(config.VolumeCheckDuration) * time.Second)
	for {
		<-c
		err := checkVolumeStatus(config)
		if err != nil {
			log.ErrorS("sche", "schedule: ", err.Error())
		}
	}
}

func checkVolumeStatus(config *util.WeederConfig) error {
	log.DebugS("sche", "schedule job running... ", config.VolumeCheckUrl)
	req, err := http.NewRequest("GET", config.VolumeCheckUrl, nil)
	if err != nil {
		return err
	}
	req.Close = true
	var httpClient http.Client
	var resp *http.Response
	resp, err = httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result []byte
	result, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.DebugT("sche", string(result))
	var topo = &SeaweedFsTopo{}
	err = json.Unmarshal(result, topo)
	if err != nil {
		return err
	}
	//	log.DebugT("sche", topo.Version)
	//	node := topo.Topology.DataCenters[0].Racks[0].DataNodes[0]
	//	log.DebugT("sche", node.Url, " ", node.Free)
	for _, dc := range topo.Topology.DataCenters {
		dcId := dc.Id
		for _, rk := range dc.Racks {
			rkId := rk.Id
			counter := 0
			freeVolumes := 0
			for _, node := range rk.DataNodes {
				if node.Free > config.VolumeCheckBaseLine {
					counter++
				}
				if freeVolumes < node.Free {
					freeVolumes = node.Free
				}
				log.DebugT("sche", "influx:dataCenter=", dcId, ",rack=", rkId,
					",remoteAddr=\"", node.Url, "\" maxVolumes=", node.Max,
					",freeVolumes=", node.Free, ",volumes=", node.Volumes,
					",value=1 ", time.Now().Unix(), "000000000")
			}
			if counter < config.NodeCheckBaseLine {
				log.DebugT("sche", "influx-alert:dataCenter=", dcId, ",rack=", rkId,
					" alertNodes=", counter,
					",alertVolumes=", config.VolumeCheckBaseLine,
					",freeVolumes=", freeVolumes,
					",value=1 ", time.Now().Unix(), "000000000")
			}
		}
	}
	return nil
}
