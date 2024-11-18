package ante_test

import (
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/evmos/ethermint/app/ante"
	"github.com/evmos/ethermint/tests"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

var evmMinGasPriceDecoratorExecTypes = []struct {
	name        string
	isCheckTx   bool
	isReCheckTx bool
	simulate    bool
}{
	{"checkTx", true, false, false},
	{"simulate", false, false, true},
	{"recheckTx", true, true, false},
	{"deliverTx", false, false, false},
}

func (s AnteTestSuite) TestEvmMinGasPriceDecoratorExecType() {
	testCases := []struct {
		name         string
		minGasPrices sdk.DecCoins
		malleate     func() sdk.Tx
		expPass      []bool
		errMsg       []string
	}{
		{
			"invalid tx type",
			[]sdk.DecCoin{{Denom: evmtypes.DefaultEVMDenom, Amount: sdk.NewInt(10).ToLegacyDec()}},
			func() sdk.Tx {
				return &invalidTx{}
			},
			[]bool{false, true, true, true},
			[]string{"invalid message type", "", "", ""},
		},
	}

	for i, et := range evmMinGasPriceDecoratorExecTypes {
		for _, tc := range testCases {
			s.Run(et.name+"_"+tc.name, func() {
				// s.SetupTest(et.isCheckTx)
				s.SetupTest()
				if et.isCheckTx {
					s.ctx = s.ctx.WithIsCheckTx(true)
					if et.isReCheckTx {
						s.ctx = s.ctx.WithIsReCheckTx(true)
					}
				}

				dec := ante.NewEvmMinGasPriceDecorator(s.app.EvmKeeper)
				s.ctx.WithMinGasPrices(tc.minGasPrices)
				_, err := dec.AnteHandle(s.ctx, tc.malleate(), et.simulate, NextFn)

				if tc.expPass[i] {
					s.Require().NoError(err, tc.name)
				} else {
					s.Require().Error(err, tc.name)
					s.Require().Contains(err.Error(), tc.errMsg[i], tc.name)
				}
			})
		}
	}
}

func (s AnteTestSuite) TestEvmMinGasPriceDecorator() {
	from, privKey := tests.NewAddrKey()
	to := tests.GenerateAddress()
	// emptyAccessList := ethtypes.AccessList{}

	minGasPrices := []sdk.DecCoin{{Denom: evmtypes.DefaultEVMDenom, Amount: sdk.NewInt(10).ToLegacyDec()}}

	testCases := []struct {
		name         string
		minGasPrices sdk.DecCoins
		malleate     func() sdk.Tx
		expPass      bool
		errMsg       string
	}{
		{
			"invalid tx type",
			minGasPrices,
			func() sdk.Tx {
				return &invalidTx{}
			},
			false,
			"invalid message type",
		},
		{
			"wrong tx type",
			minGasPrices,
			func() sdk.Tx {
				testMsg := banktypes.MsgSend{
					FromAddress: "evmos1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0ptzkucp",
					ToAddress:   "evmos1dx67l23hz9l0k9hcher8xz04uj7wf3yu26l2yn",
					Amount:      sdk.Coins{sdk.Coin{Amount: sdkmath.NewInt(10), Denom: evmtypes.DefaultEVMDenom}},
				}
				txBuilder := s.CreateTestCosmosTxBuilder(sdkmath.NewInt(0), evmtypes.DefaultEVMDenom, &testMsg)
				return txBuilder.GetTx()
			},
			false,
			"invalid message type",
		},
		{
			"empty min gas prices",
			[]sdk.DecCoin{},
			func() sdk.Tx {
				return &invalidTx{}
			},
			true,
			"",
		},
		{
			"min gas prices < 0",
			[]sdk.DecCoin{{Denom: evmtypes.DefaultEVMDenom, Amount: sdk.NewInt(-10).ToLegacyDec()}},
			func() sdk.Tx {
				msg := s.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(20), nil, nil, nil)
				return s.CreateTestTx(msg, privKey, 1, false)
			},
			false,
			"invalid min gas price for",
		},
		{
			"min gas prices = 0",
			[]sdk.DecCoin{{Denom: evmtypes.DefaultEVMDenom, Amount: sdk.NewInt(0).ToLegacyDec()}},
			func() sdk.Tx {
				msg := s.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(20), nil, nil, nil)
				return s.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"invalid legacy tx with gasPrice = 0",
			minGasPrices,
			func() sdk.Tx {
				msg := s.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(0), nil, nil, nil)
				return s.CreateTestTx(msg, privKey, 1, false)
			},
			false,
			"less than minimum global gas price",
		},
		{
			"valid legacy tx with gasPrice = 0",
			minGasPrices,
			func() sdk.Tx {
				msg := s.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(10), nil, nil, nil)
				return s.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
		{
			"valid legacy tx with gasPrice > 0",
			minGasPrices,
			func() sdk.Tx {
				msg := s.BuildTestEthTx(from, to, nil, make([]byte, 0), big.NewInt(20), nil, nil, nil)
				return s.CreateTestTx(msg, privKey, 1, false)
			},
			true,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.ctx = s.ctx.WithIsCheckTx(true)
			s.ctx = s.ctx.WithMinGasPrices([]sdk.DecCoin{})
			dec := ante.NewEvmMinGasPriceDecorator(s.app.EvmKeeper)
			s.ctx = s.ctx.WithMinGasPrices(tc.minGasPrices)
			_, err := dec.AnteHandle(s.ctx, tc.malleate(), false, NextFn)

			if tc.expPass {
				s.Require().NoError(err, tc.name)
			} else {
				s.Require().Error(err, tc.name)
				s.Require().Contains(err.Error(), tc.errMsg, tc.name)
			}
		})
	}
}
