package ante

import (
	"math/big"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

type EvmMinGasPriceDecorator struct {
	evmKeeper EVMKeeper
	evmDenom  string
}

func NewEvmMinGasPriceDecorator(ek EVMKeeper) *EvmMinGasPriceDecorator {
	return &EvmMinGasPriceDecorator{
		evmKeeper: ek,
	}
}

func (mgp *EvmMinGasPriceDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	if !ctx.IsCheckTx() || ctx.IsReCheckTx() {
		return next(ctx, tx, simulate)
	}

	minGasPrices := ctx.MinGasPrices()

	// Short-circuit if min gas price is 0 or if simulating
	if minGasPrices.Empty() || simulate {
		return next(ctx, tx, simulate)
	}

	if len(mgp.evmDenom) == 0 {
		evmParams := mgp.evmKeeper.GetParams(ctx)
		mgp.evmDenom = evmParams.GetEvmDenom()
	}

	evmMinGasPrice := big.NewInt(0)

	for _, gp := range minGasPrices {
		if gp.Denom == mgp.evmDenom {
			evmMinGasPrice.SetString(gp.Amount.String(), 10)
			break
		}
	}

	if evmMinGasPrice.Sign() < 0 {
		return ctx, errorsmod.Wrapf(errortypes.ErrInvalidCoins, "invalid min gas price for %s", mgp.evmDenom)
	} else if evmMinGasPrice.Sign() == 0 {
		// Short-circuit if min gas price is 0
		return next(ctx, tx, simulate)
	}

	for i, msg := range tx.GetMsgs() {
		msgEthTx, ok := msg.(*evmtypes.MsgEthereumTx)
		if !ok {
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid message type %T, expected %T", msg, (*evmtypes.MsgEthereumTx)(nil))
		}

		txData, err := evmtypes.UnpackTxData(msgEthTx.Data)
		if err != nil {
			return ctx, errorsmod.Wrapf(err, "failed to unpack tx data any for tx %d", i)
		}

		txGasPrice := txData.GetGasPrice()

		if txGasPrice.Cmp(evmMinGasPrice) < 0 {
			return ctx, errorsmod.Wrapf(errortypes.ErrInsufficientFee, "tx gas price (%s) is less than minimum global gas price (%s)", txGasPrice, evmMinGasPrice)
		}
	}

	return next(ctx, tx, simulate)
}
