package main

import (
	"flag"
	"github.com/qerio/goctp"
	"log"
	"os"
	"reflect"
	"time"
)

var (
	broker_id    = flag.String("BrokerID", "9999", "经纪公司编号,SimNow BrokerID统一为：9999")
	investor_id  = flag.String("InvestorID", "<InvestorID>", "交易用户代码")
	pass_word    = flag.String("Password", "<Password>", "交易用户密码")
	market_front = flag.String("MarketFront", "tcp://180.168.146.187:10031", "行情前置,SimNow的测试环境: tcp://180.168.146.187:10031")
	trade_front  = flag.String("TradeFront", "tcp://180.168.146.187:10030", "交易前置,SimNow的测试环境: tcp://180.168.146.187:10030")
)

//全局变量非类成员，仅供测试使用
var (
	tradingDate string
	FrontID     int
	SessionID   int
)

var CTP GoCTPClient

type GoCTPClient struct {
	BrokerID   string
	InvestorID string
	Password   string

	MdFront string
	MdApi   goctp.CThostFtdcMdApi

	TraderFront string
	TraderApi   goctp.CThostFtdcTraderApi

	MdRequestID     int
	TraderRequestID int
}

func (g *GoCTPClient) GetMdRequestID() int {
	g.MdRequestID += 1
	return g.MdRequestID
}

func (g *GoCTPClient) GetTraderRequestID() int {
	g.TraderRequestID += 1
	return g.TraderRequestID
}

type GoCThostFtdcTraderSpi struct {
	Client GoCTPClient
}

func (p *GoCThostFtdcTraderSpi) IsErrorRspInfo(pRspInfo goctp.CThostFtdcRspInfoField) bool {

	iResult := (pRspInfo.GetErrorID() != 0)

	if iResult && pRspInfo.GetErrorID() != 0 {
		log.Printf("ErrorID=%v ErrorMsg=%v\n", pRspInfo.GetErrorID(), pRspInfo.GetErrorMsg())
	}
	return iResult
}

///判断接口内容为空
func (p *GoCThostFtdcTraderSpi) isEmpty(a interface{}) bool {
	v := reflect.ValueOf(a)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v.Interface() == reflect.Zero(v.Type()).Interface()
}

func (p *GoCThostFtdcTraderSpi) OnRspError(pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspError.")
	p.IsErrorRspInfo(pRspInfo)
}

func (p *GoCThostFtdcTraderSpi) OnFrontDisconnected(nReason int) {
	log.Printf("GoCThostFtdcTraderSpi.OnFrontDisconnected: %#v", nReason)
}

func (p *GoCThostFtdcTraderSpi) OnHeartBeatWarning(nTimeLapse int) {
	log.Printf("GoCThostFtdcTraderSpi.OnHeartBeatWarning: %#v", nTimeLapse)
}

func (p *GoCThostFtdcTraderSpi) OnFrontConnected() {
	log.Println("GoCThostFtdcTraderSpi.OnFrontConnected.")
	p.ReqUserLogin()
}

func (p *GoCThostFtdcTraderSpi) ReqUserLogin() {
	log.Println("GoCThostFtdcTraderSpi.ReqUserLogin.")

	req := goctp.NewCThostFtdcReqUserLoginField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetUserID(p.Client.InvestorID)
	req.SetPassword(p.Client.Password)

	iResult := p.Client.TraderApi.ReqUserLogin(req, p.Client.GetTraderRequestID())

	if iResult != 0 {
		log.Println("发送用户登录请求: 失败.")
	} else {
		log.Println("发送用户登录请求: 成功.")
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspUserLogin(pRspUserLogin goctp.CThostFtdcRspUserLoginField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {

	log.Println("GoCThostFtdcTraderSpi.OnRspUserLogin.")
	if bIsLast && !p.IsErrorRspInfo(pRspInfo) {
		tradingDate = pRspUserLogin.GetTradingDay()
		FrontID = pRspUserLogin.GetFrontID()
		SessionID = pRspUserLogin.GetSessionID()
		log.Printf("获取当前交易日: %#v\n", pRspUserLogin.GetTradingDay())
		log.Printf("获取用户登录信息: %#v %#v %#v\n", FrontID, SessionID, pRspUserLogin.GetMaxOrderRef())

		///投资者结算结果确认
		p.ReqQrySettlementInfoConfirm()
	}
}

func (p *GoCThostFtdcTraderSpi) ReqQrySettlementInfoConfirm() {
	req := goctp.NewCThostFtdcQrySettlementInfoConfirmField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	for {
		iResult := p.Client.TraderApi.ReqQrySettlementInfoConfirm(req, p.Client.GetTraderRequestID())
		if iResult == 0 {
			log.Println("请求查询结算单确认日期: 成功, iResult=", iResult)
			break
		} else {
			log.Println("请求查询结算单确认日期: 失败, iResult=", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspQrySettlementInfoConfirm(pSettlementInfoConfirm goctp.CThostFtdcSettlementInfoConfirmField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQrySettlementInfoConfirm.")
	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if !p.isEmpty(pSettlementInfoConfirm) {
			log.Println(pSettlementInfoConfirm.GetConfirmDate())
			log.Println(pSettlementInfoConfirm.GetConfirmTime())

			lastConfirmData := pSettlementInfoConfirm.GetConfirmDate()
			if lastConfirmData != tradingDate {
				p.ReqQrySettlementInfo()
			} else {
				log.Println("添加想要查询或执行的操作")
				///添加想要查询或执行的操作
				//p.ReqQryInvestorPosition("")
				//p.ReqQryOrder()
			}
		} else {
			p.ReqQrySettlementInfo()
		}

	}
}

func (p *GoCThostFtdcTraderSpi) ReqQrySettlementInfo() {
	req := goctp.NewCThostFtdcQrySettlementInfoField()

	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)

	for {
		iResult := p.Client.TraderApi.ReqQrySettlementInfo(req, p.Client.GetTraderRequestID())
		if iResult == 0 {
			log.Println("请求查询结算单: 成功, iResult=", iResult)
			break
		} else {
			log.Println("请求查询结算单: 失败, iResult=", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspQrySettlementInfo(pSettlementInfo goctp.CThostFtdcSettlementInfoField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQrySettlementInfo.")
	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {

		if !p.isEmpty(pSettlementInfo) {
			log.Println("chaxunjiesuandan")
		}
		//确认结算单
		p.ReqSettlementInfoConfirm()
	}
}

func (p *GoCThostFtdcTraderSpi) ReqSettlementInfoConfirm() {
	req := goctp.NewCThostFtdcSettlementInfoConfirmField()

	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)

	iResult := p.Client.TraderApi.ReqSettlementInfoConfirm(req, p.Client.GetTraderRequestID())

	if iResult == 0 {
		log.Println("投资者结算结果确认: 成功, iResult=", iResult)
	} else {
		log.Println("投资者结算结果确认: 失败, iResult=", iResult)
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspSettlementInfoConfirm(pSettlementInfoConfirm goctp.CThostFtdcSettlementInfoConfirmField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {

	log.Println("GoCThostFtdcTraderSpi.OnRspSettlementInfoConfirm.")
	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if !p.isEmpty(pSettlementInfoConfirm) {
			log.Println("ConfirmTime: ", pSettlementInfoConfirm.GetConfirmTime())
		}
		log.Println("添加想要查询或执行的操作")
		///添加想要的查询或执行的操作
		p.ReqQryTradingAccount()
	}
}

///p.ReqQryInvestorPosition("")空字符串表示查询所有
func (p *GoCThostFtdcTraderSpi) ReqQryInvestorPosition(InstrumentID string) {

	req := goctp.NewCThostFtdcQryInvestorPositionField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetInstrumentID(InstrumentID)

	for {
		iResult := p.Client.TraderApi.ReqQryInvestorPosition(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> 请求查询投资者持仓: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> 请求查询投资者持仓: 失败 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspQryInvestorPosition(pInvestorPosition goctp.CThostFtdcInvestorPositionField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQryInvestorPosition.")

	log.Printf("OnRspQryInvestorPosition ID: %#v", nRequestID)
	//p.ReqOrderInsert()
	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if !p.isEmpty(pInvestorPosition) {
			log.Println("YdPosition:", pInvestorPosition.GetYdPosition())
			log.Println("Position:", pInvestorPosition.GetPosition())
			log.Println("InstrumentID:", pInvestorPosition.GetInstrumentID())
		} else {
			log.Println("kong")
		}
	}
}


func (p *GoCThostFtdcTraderSpi) ReqQryInvestorPositionDetail(InstrumentID string) {

	req := goctp.NewCThostFtdcQryInvestorPositionDetailField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetInstrumentID(InstrumentID)

	for {
		iResult := p.Client.TraderApi.ReqQryInvestorPositionDetail(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> 请求查询投资者持仓详情: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> 请求查询投资者持仓详情: 失败 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspQryInvestorPositionDetail(pInvestorPosition goctp.CThostFtdcInvestorPositionDetailField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQryInvestorPositionDetail.")

	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if !p.isEmpty(pInvestorPosition) {
			log.Println("InstrumentID:", pInvestorPosition.GetInstrumentID())
			log.Println("TradeID:", pInvestorPosition.GetTradeID())
		} else {
			log.Println("kong")
		}
	}
}

//未测试组合报单
func (p *GoCThostFtdcTraderSpi) ReqQryInvestorPositionCombineDetail() {

	req := goctp.NewCThostFtdcQryInvestorPositionCombineDetailField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	//req.SetCombInstrumentID("")

	for {
		iResult := p.Client.TraderApi.ReqQryInvestorPositionCombineDetail(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> 请求查询投资者组合持仓详情: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> 请求查询投资者组合持仓详情: 失败 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspQryInvestorPositionCombineDetail(pInvestorPosition goctp.CThostFtdcInvestorPositionCombineDetailField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQryInvestorPositionCombineDetail.")

	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if !p.isEmpty(pInvestorPosition) {
			log.Println("ComTradeID:", pInvestorPosition.GetComTradeID())
			log.Println("TradeID:", pInvestorPosition.GetTradeID())
			log.Println("InstrumentID:", pInvestorPosition.GetInstrumentID())
		} else {
			log.Println("kong")
		}
	}
}

func (p *GoCThostFtdcTraderSpi) ReqQryInstrument() {
	req := goctp.NewCThostFtdcQryInstrumentField()

	var id string = "cu1709"
	req.SetInstrumentID(id)

	for {
		iResult := p.Client.TraderApi.ReqQryInstrument(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> 请求查询合约: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> 请求查询合约: 失败 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspQryInstrument(pInstrument goctp.CThostFtdcInstrumentField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQryInstrument.")
	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if !p.isEmpty(pInstrument) {
			log.Println("GoCThostFtdcTraderSpi.OnRspQryInstrument: ", pInstrument.GetInstrumentID(), pInstrument.GetExchangeID(),
				pInstrument.GetInstrumentName(), pInstrument.GetExchangeInstID(), pInstrument.GetProductID(), pInstrument.GetProductClass(),
				pInstrument.GetDeliveryYear(), pInstrument.GetDeliveryMonth(), pInstrument.GetMaxMarketOrderVolume(), pInstrument.GetMinMarketOrderVolume(),
				pInstrument.GetMaxLimitOrderVolume(), pInstrument.GetMinLimitOrderVolume(), pInstrument.GetVolumeMultiple(), pInstrument.GetPriceTick(),
				pInstrument.GetCreateDate(), pInstrument.GetOpenDate(), pInstrument.GetExpireDate(), pInstrument.GetStartDelivDate(), pInstrument.GetEndDelivDate())
		} else {
			log.Println("kong")
		}
	}
}

func (p *GoCThostFtdcTraderSpi) ReqQryTradingAccount() {
	req := goctp.NewCThostFtdcQryTradingAccountField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)

	for {
		iResult := p.Client.TraderApi.ReqQryTradingAccount(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> 请求查询资金账户: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> 请求查询资金账户: 失败 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspQryTradingAccount(pTradingAccount goctp.CThostFtdcTradingAccountField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {

	log.Println("GoCThostFtdcTraderSpi.OnRspQryTradingAccount.")

	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if p.isEmpty(pTradingAccount) {
			log.Printf("Commission:%#v", pTradingAccount.GetCommission())
		} else {
			log.Println("kong")
		}
	}
}

//插入报单
func (p *GoCThostFtdcTraderSpi) ReqOrderInsert() {
	req := goctp.NewCThostFtdcInputOrderField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetInstrumentID("rb1709")
	req.SetDirection(goctp.THOST_FTDC_D_Buy)
	req.SetCombOffsetFlag(string(goctp.THOST_FTDC_OF_Open))
	req.SetCombHedgeFlag(string(goctp.THOST_FTDC_HF_Speculation))
	req.SetVolumeTotalOriginal(1)
	req.SetContingentCondition(goctp.THOST_FTDC_CC_Immediately)
	req.SetVolumeCondition(goctp.THOST_FTDC_VC_AV)
	req.SetMinVolume(1)
	req.SetForceCloseReason(goctp.THOST_FTDC_FCC_NotForceClose)
	req.SetIsAutoSuspend(0)
	req.SetUserForceClose(0)
	req.SetOrderPriceType(goctp.THOST_FTDC_OPT_LimitPrice)
	req.SetLimitPrice(3200.00)
	req.SetTimeCondition(goctp.THOST_FTDC_TC_GFD)

	iResult := p.Client.TraderApi.ReqOrderInsert(req, p.Client.GetTraderRequestID())

	if iResult == 0 {
		log.Println("报单插入: 成功, iResult=", iResult)
	} else {
		log.Println("报单插入: 失败, iResult=", iResult)
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspOrderInsert(pInputOrder goctp.CThostFtdcInputOrderField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspOrderInsert.")

	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		log.Println(1)
	}
}

func (p *GoCThostFtdcTraderSpi) OnErrRtnOrderInsert(pInputOrder goctp.CThostFtdcInputOrderField, pRspInfo goctp.CThostFtdcRspInfoField) {
	log.Println("GoCThostFtdcTraderSpi.OnErrRtnOrderInsert.")

	if !p.isEmpty(pRspInfo) && !p.IsErrorRspInfo(pRspInfo) {
		log.Println(2)
	}
}

func (p *GoCThostFtdcTraderSpi) OnRtnOrder(pOrder goctp.CThostFtdcOrderField) {
	log.Println("GoCThostFtdcTraderSpi.OnRtnOrder.")
	log.Println("CancelTime:", pOrder.GetCancelTime())
	log.Println("交易所编号:", pOrder.GetExchangeID())
	log.Println("合约代码:", pOrder.GetInstrumentID())
	log.Println("FrontID:", pOrder.GetFrontID())
	log.Println("SessionID:", uint32(pOrder.GetSessionID()))
	log.Println("报单引用:", pOrder.GetOrderRef())
	log.Println("买卖方向:", pOrder.GetDirection())
	log.Println("组合开平标志:", pOrder.GetCombOffsetFlag())
	log.Println("价格:", pOrder.GetLimitPrice())
	log.Println("数量:", pOrder.GetVolumeTotalOriginal())
	log.Println("今成交数量:", pOrder.GetVolumeTraded())
	log.Println("剩余数量:", pOrder.GetVolumeTotal())
	log.Println("报单编号（判断报单是否有效）:", pOrder.GetOrderSysID())
	//log.Println("OrderSubmitStatus:", string(pOrder.GetOrderSubmitStatus()))
	//log.Println("报单状态:", string(pOrder.GetOrderStatus()))
	log.Println("OrderSubmitStatus:", pOrder.GetOrderSubmitStatus())
	log.Println("报单状态:", pOrder.GetOrderStatus())
	log.Println("报单日期:", pOrder.GetInsertDate())
	log.Println("序号:", pOrder.GetSequenceNo())
}

func (p *GoCThostFtdcTraderSpi) OnRtnTrade(pTrade goctp.CThostFtdcTradeField) {
	log.Println("GoCThostFtdcTraderSpi.OnRtnTrade.")
}

//撤单
func (p *GoCThostFtdcTraderSpi) ReqOrderAction() {
	log.Println("GoCThostFtdcTraderSpi.ReqOrderAction.")
	req := goctp.NewCThostFtdcInputOrderActionField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetExchangeID("SHFE")
	//req.SetOrderSysID(OrderSysID)
	req.SetActionFlag(goctp.THOST_FTDC_AF_Delete)

	iResult := p.Client.TraderApi.ReqOrderAction(req, p.Client.GetTraderRequestID())

	if iResult != 0 {
		log.Println("ReqOrderAction: 失败.")
	} else {
		log.Println("ReqOrderAction: 成功.")
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspOrderAction(pInputOrderAction goctp.CThostFtdcInputOrderActionField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspOrderInsert.")

	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		log.Println("1234")
	}
}

func (p *GoCThostFtdcTraderSpi) OnErrRtnOrderAction(pInputOrderAction goctp.CThostFtdcInputOrderActionField, pRspInfo goctp.CThostFtdcRspInfoField) {
	log.Println("GoCThostFtdcTraderSpi.OnErrRtnOrderInsert.")

	if p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo) {
		log.Println("2")
	}
}

func (p *GoCThostFtdcTraderSpi) ReqParkedOrderInsert() {
	req := goctp.NewCThostFtdcParkedOrderField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetInstrumentID("rb1709")
	req.SetDirection(goctp.THOST_FTDC_D_Buy)
	req.SetCombOffsetFlag(string(goctp.THOST_FTDC_OF_Open))
	req.SetCombHedgeFlag(string(goctp.THOST_FTDC_HF_Speculation))
	req.SetVolumeTotalOriginal(1)
	req.SetContingentCondition(goctp.THOST_FTDC_CC_Immediately)
	req.SetVolumeCondition(goctp.THOST_FTDC_VC_AV)
	req.SetMinVolume(1)
	req.SetForceCloseReason(goctp.THOST_FTDC_FCC_NotForceClose)
	req.SetIsAutoSuspend(0)
	req.SetUserForceClose(0)
	req.SetOrderPriceType(goctp.THOST_FTDC_OPT_LimitPrice)
	req.SetLimitPrice(3245.00)
	req.SetTimeCondition(goctp.THOST_FTDC_TC_GFD)

	iResult := p.Client.TraderApi.ReqParkedOrderInsert(req, p.Client.GetTraderRequestID())

	if iResult != 0 {
		log.Println("reqParkedOrderInsert: 失败.")
	} else {
		log.Println("reqParkedOrderInsert: 成功.")
	}
}

func (p *GoCThostFtdcTraderSpi) OnRspParkedOrderInsert(pParkedOrder goctp.CThostFtdcParkedOrderField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspParkedOrderInsert.")

	if bIsLast && !p.isEmpty(pRspInfo) && !p.IsErrorRspInfo(pRspInfo) {
		log.Println("GoCThostFtdcTraderSpi.OnRtnOrder.")
		log.Println("交易所编号:", pParkedOrder.GetExchangeID())
		log.Println("合约代码:", pParkedOrder.GetInstrumentID())
		log.Println("报单引用:", pParkedOrder.GetOrderRef())
		log.Println("买卖方向:", pParkedOrder.GetDirection())
		log.Println("组合开平标志:", pParkedOrder.GetCombOffsetFlag())
		log.Println("价格:", pParkedOrder.GetLimitPrice())
		log.Println("数量:", pParkedOrder.GetVolumeTotalOriginal())
		log.Println("ParkedOrderID:", pParkedOrder.GetParkedOrderID())
		log.Println("Status:", pParkedOrder.GetStatus())
	}
}

///以上均已测试
///未测试
func (p *GoCThostFtdcTraderSpi) ReqParkedOrderAction() {
	req := goctp.NewCThostFtdcParkedOrderActionField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	//req.SetExchangeID("SHFE")
	//req.SetOrderSysID(******)
	req.SetActionFlag(goctp.THOST_FTDC_AF_Delete)

	iResult := p.Client.TraderApi.ReqParkedOrderAction(req, p.Client.GetTraderRequestID())

	if iResult != 0 {
		log.Println("ReqParkedOrderAction: 失败.")
	} else {
		log.Println("ReqParkedOrderAction: 成功.")
	}
}

///未测试
func (p *GoCThostFtdcTraderSpi) OnRspParkedOrderAction(pParkedOrderAction goctp.CThostFtdcParkedOrderActionField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspParkedOrderAction.")

	if bIsLast && !p.isEmpty(pRspInfo) && !p.IsErrorRspInfo(pRspInfo) {
		log.Println("1")
	}
}

///未测试
func (p *GoCThostFtdcTraderSpi) ReqQryParkedOrder() {

	req := goctp.NewCThostFtdcQryParkedOrderField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetInstrumentID("rb1709")
	req.SetExchangeID("SHFE")

	for {
		iResult := p.Client.TraderApi.ReqQryParkedOrder(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> ReqQryParkedOrder: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> ReqQryParkedOrder: 受到流控 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

///未测试
func (p *GoCThostFtdcTraderSpi) OnRspQryParkedOrder(pInvestorPosition goctp.CThostFtdcParkedOrderField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQryParkedOrder.")

	if bIsLast && !p.isEmpty(pRspInfo) && !p.IsErrorRspInfo(pRspInfo) {
		log.Printf("1")
	}
}

///未测试
func (p *GoCThostFtdcTraderSpi) ReqQryParkedOrderAction() {

	req := goctp.NewCThostFtdcQryParkedOrderActionField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetInstrumentID("rb1709")
	req.SetExchangeID("SHFE")

	for {
		iResult := p.Client.TraderApi.ReqQryParkedOrderAction(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> ReqQryParkedOrderAction: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> ReqQryParkedOrderAction: 受到流控 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

///未测试
func (p *GoCThostFtdcTraderSpi) OnRspQryParkedOrderAction(pInvestorPosition goctp.CThostFtdcParkedOrderActionField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspQryParkedOrderAction.")

	if bIsLast && !p.isEmpty(pRspInfo) && !p.IsErrorRspInfo(pRspInfo) {
		log.Printf("1")
	}
}

///已测试
func (p *GoCThostFtdcTraderSpi) ReqRemoveParkedOrder() {

	req := goctp.NewCThostFtdcRemoveParkedOrderField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	//req.SetParkedOrderID("ParkedOrderID")

	for {
		iResult := p.Client.TraderApi.ReqRemoveParkedOrder(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> ReqRemoveParkedOrder: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> ReqRemoveParkedOrder: 受到流控 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

///已测试
func (p *GoCThostFtdcTraderSpi) OnRspRemoveParkedOrder(pRemoveParkedOrder goctp.CThostFtdcRemoveParkedOrderField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspRemoveParkedOrder.")

	if bIsLast && !p.isEmpty(pRspInfo) && !p.IsErrorRspInfo(pRspInfo) {
		if !p.isEmpty(pRemoveParkedOrder) {
			log.Printf("ok1")
		}

	}
}

///已测试
func (p *GoCThostFtdcTraderSpi) ReqRemoveParkedOrderAction() {

	req := goctp.NewCThostFtdcRemoveParkedOrderActionField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	//req.SetParkedOrderID("ParkedOrderActionID")

	for {
		iResult := p.Client.TraderApi.ReqRemoveParkedOrderAction(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> ReqRemoveParkedOrderAction: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> ReqRemoveParkedOrderAction: 受到流控 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

///已测试
func (p *GoCThostFtdcTraderSpi) OnRspRemoveParkedOrderAction(pRemoveParkedOrderAction goctp.CThostFtdcRemoveParkedOrderActionField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	log.Println("GoCThostFtdcTraderSpi.OnRspRemoveParkedOrderAction.")

	if bIsLast && !p.isEmpty(pRspInfo) && !p.IsErrorRspInfo(pRspInfo) {
		if !p.isEmpty(pRemoveParkedOrderAction) {
			log.Printf("ok2")
		}

	}
}

func (p *GoCThostFtdcTraderSpi) ReqQryOrder() {
	req := goctp.NewCThostFtdcQryOrderField()
	req.SetBrokerID(p.Client.BrokerID)
	req.SetInvestorID(p.Client.InvestorID)
	req.SetExchangeID("SHFE")
	req.SetOrderSysID("")

	for {
		iResult := p.Client.TraderApi.ReqQryOrder(req, p.Client.GetTraderRequestID())

		if iResult == 0 {
			log.Printf("--->>> ReqQryOrder: 成功 %#v\n", iResult)
			break
		} else {
			log.Printf("--->>> ReqQryOrder: 失败 %#v\n", iResult)
			time.Sleep(1 * time.Second)
		}
	}
}

//测试失败无法通过这种方式获得唯一编号
func (p *GoCThostFtdcTraderSpi) OnRspQryOrder(pOrder goctp.CThostFtdcOrderField, pRspInfo goctp.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {

	log.Println("GoCThostFtdcTraderSpi.OnRspQryOrder.")

	if bIsLast && (p.isEmpty(pRspInfo) || !p.IsErrorRspInfo(pRspInfo)) {
		if p.isEmpty(pOrder) {
			log.Printf("OrderStatus:%#v", pOrder.GetOrderStatus())
			log.Printf("OrderSysID:%#v", pOrder.GetOrderSysID())
		} else {
			log.Println("kong")
		}
	}
}

func init() {
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("CTP: ")
}

func main() {

	if len(os.Args) < 2 {
		log.Fatal("usage: ./goctp_trader_example -BrokerID 9999 -InvestorID 000000 -Password 000000 -MarketFront tcp://180.168.146.187:10010 -TradeFront tcp://180.168.146.187:10000")
	}

	flag.Parse()

	CTP = GoCTPClient{
		BrokerID:   *broker_id,
		InvestorID: *investor_id,
		Password:   *pass_word,

		MdFront: *market_front,
		MdApi:   goctp.CThostFtdcMdApiCreateFtdcMdApi(),

		TraderFront: *trade_front,
		TraderApi:   goctp.CThostFtdcTraderApiCreateFtdcTraderApi(),

		MdRequestID:     0,
		TraderRequestID: 0,
	}

	pTraderSpi := goctp.NewDirectorCThostFtdcTraderSpi(&GoCThostFtdcTraderSpi{Client: CTP})

	CTP.TraderApi.RegisterSpi(pTraderSpi)                         // 注册事件类
	CTP.TraderApi.SubscribePublicTopic(1 /*THOST_TERT_RESTART*/)  // 注册公有流
	CTP.TraderApi.SubscribePrivateTopic(1 /*THOST_TERT_RESTART*/) // 注册私有流
	CTP.TraderApi.RegisterFront(CTP.TraderFront)
	CTP.TraderApi.Init()

	CTP.TraderApi.Join()
	CTP.TraderApi.Release()
}
