package model

type StatusId uint8

type Status struct {
	Id          StatusId `json:"id"`
	Description string   `json:"description"`
}

const (
	StatusPD        StatusId = 0
	StatusIQ        StatusId = 1
	StatusPR        StatusId = 2
	StatusAC        StatusId = 3
	StatusWA        StatusId = 4
	StatusTLE       StatusId = 5
	StatusCE        StatusId = 6
	StatusRESIGSEGV StatusId = 7
	StatusRESIGXFSZ StatusId = 8
	StatusRESIGFPE  StatusId = 9
	StatusRESIGABRT StatusId = 10
	StatusRENZEC    StatusId = 11
	StatusRE        StatusId = 12
	StatusIE        StatusId = 13
	StatusEFE       StatusId = 14
)

func (s StatusId) String() string {
	switch s {
	case StatusPD:
		return "Pending"
	case StatusIQ:
		return "In Queue"
	case StatusPR:
		return "Processing"
	case StatusAC:
		return "Accepted"
	case StatusWA:
		return "Wrong Answer"
	case StatusTLE:
		return "Time Limit Exceeded"
	case StatusCE:
		return "Compilation Error"
	case StatusRESIGSEGV:
		return "Runtime Error (SIGSEGV)"
	case StatusRESIGXFSZ:
		return "Runtime Error (SIGXFSZ)"
	case StatusRESIGFPE:
		return "Runtime Error (SIGFPE)"
	case StatusRESIGABRT:
		return "Runtime Error (SIGABRT)"
	case StatusRENZEC:
		return "Runtime Error (NZEC)"
	case StatusRE:
		return "Runtime Error"
	case StatusIE:
		return "Internal Error"
	case StatusEFE:
		return "Exec Format Error"
	default:
		return "Unknown"
	}
}

func (id StatusId) GetStatus() Status {
	return Status{
		Id:          id,
		Description: id.String(),
	}
}
