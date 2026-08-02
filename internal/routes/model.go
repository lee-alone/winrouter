package routes

type Route struct {
	Prefix         string `json:"prefix"`
	InterfaceLUID  uint64 `json:"interface_luid"`
	InterfaceIndex uint32 `json:"interface_index"`
	Metric         uint32 `json:"metric"`
	Protocol       uint32 `json:"protocol"`
}
