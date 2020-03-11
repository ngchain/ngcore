package rpcServer

import (
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/sheetManager"
	"net/http"
)

//type sheet

type Sheet struct {
	sm *sheetManager.SheetManager
}

func NewSheetModule(sm *sheetManager.SheetManager) *Sheet {
	return &Sheet{
		sm: sm,
	}
}

type GetCurrentSheetReply struct {
	Sheet *ngtypes.Sheet
}

func (sm *Sheet) GetCurrentSheet(r *http.Request, args *struct{}, reply *GetCurrentSheetReply) error {
	reply.Sheet = sm.sm.GenerateSheet()
	log.Info(reply.Sheet)
	return nil
}
