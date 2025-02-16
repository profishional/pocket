package utility

import (
	"bytes"
	"encoding/hex"

	"github.com/pokt-network/pocket/shared/codec"
	coreTypes "github.com/pokt-network/pocket/shared/core/types"
	"github.com/pokt-network/pocket/shared/crypto"
	"github.com/pokt-network/pocket/shared/modules"
	typesUtil "github.com/pokt-network/pocket/utility/types"
)

func (u *utilityModule) CheckTransaction(txProtoBytes []byte) error {
	// Is the tx already in the mempool (in memory)?
	txHash := typesUtil.TransactionHash(txProtoBytes)
	if u.Mempool.Contains(txHash) {
		return typesUtil.ErrDuplicateTransaction()
	}

	// Is the tx already indexed (on disk)?
	persistenceModule := u.GetBus().GetPersistenceModule()
	if txExists, err := persistenceModule.TransactionExists(txHash); err != nil {
		return err
	} else if txExists {
		// TODO: non-ordered nonce requires non-pruned tx indexer
		return typesUtil.ErrTransactionAlreadyCommitted()
	}

	// Can the tx bytes be decoded as a protobuf?
	transaction := &typesUtil.Transaction{}
	if err := codec.GetCodec().Unmarshal(txProtoBytes, transaction); err != nil {
		return typesUtil.ErrProtoUnmarshal(err)
	}

	// Does the tx pass basic validation?
	if err := transaction.ValidateBasic(); err != nil {
		return err
	}

	// Store the tx in the mempool
	return u.Mempool.AddTransaction(txProtoBytes)
}

func (u *UtilityContext) ApplyTransaction(index int, tx *typesUtil.Transaction) (modules.TxResult, typesUtil.Error) {
	msg, signer, err := u.AnteHandleMessage(tx)
	if err != nil {
		return nil, err
	}
	return tx.ToTxResult(u.Height, index, signer, msg.GetMessageRecipient(), msg.GetMessageName(), u.HandleMessage(msg))
}

// CLEANUP: Exposed for testing purposes only
func (u *UtilityContext) AnteHandleMessage(tx *typesUtil.Transaction) (msg typesUtil.Message, signer string, err typesUtil.Error) {
	msg, err = tx.Message()
	if err != nil {
		return nil, "", err
	}
	fee, err := u.GetFee(msg, msg.GetActorType())
	if err != nil {
		return nil, "", err
	}
	pubKey, er := crypto.NewPublicKeyFromBytes(tx.Signature.PublicKey)
	if er != nil {
		return nil, "", typesUtil.ErrNewPublicKeyFromBytes(er)
	}
	address := pubKey.Address()
	accountAmount, err := u.GetAccountAmount(address)
	if err != nil {
		return nil, "", typesUtil.ErrGetAccountAmount(err)
	}
	accountAmount.Sub(accountAmount, fee)
	if accountAmount.Sign() == -1 {
		return nil, "", typesUtil.ErrInsufficientAmount(address.String())
	}
	signerCandidates, err := u.GetSignerCandidates(msg)
	if err != nil {
		return nil, "", err
	}
	var isValidSigner bool
	for _, candidate := range signerCandidates {
		if bytes.Equal(candidate, address) {
			isValidSigner = true
			signer = hex.EncodeToString(candidate)
			break
		}
	}
	if !isValidSigner {
		return nil, signer, typesUtil.ErrInvalidSigner()
	}
	if err := u.SetAccountAmount(address, accountAmount); err != nil {
		return nil, signer, err
	}
	if err := u.AddPoolAmount(coreTypes.Pools_POOLS_FEE_COLLECTOR.FriendlyName(), fee); err != nil {
		return nil, "", err
	}
	msg.SetSigner(address)
	return msg, signer, nil
}

func (u *UtilityContext) HandleMessage(msg typesUtil.Message) (err typesUtil.Error) {
	switch x := msg.(type) {
	case *typesUtil.MessageDoubleSign:
		return u.HandleMessageDoubleSign(x)
	case *typesUtil.MessageSend:
		return u.HandleMessageSend(x)
	case *typesUtil.MessageStake:
		return u.HandleStakeMessage(x)
	case *typesUtil.MessageEditStake:
		return u.HandleEditStakeMessage(x)
	case *typesUtil.MessageUnstake:
		return u.HandleUnstakeMessage(x)
	case *typesUtil.MessageUnpause:
		return u.HandleUnpauseMessage(x)
	case *typesUtil.MessageChangeParameter:
		return u.HandleMessageChangeParameter(x)
	default:
		return typesUtil.ErrUnknownMessage(x)
	}
}

func (u *UtilityContext) HandleMessageSend(message *typesUtil.MessageSend) typesUtil.Error {
	// convert the amount to big.Int
	amount, err := typesUtil.StringToBigInt(message.Amount)
	if err != nil {
		return err
	}
	// get the sender's account amount
	fromAccountAmount, err := u.GetAccountAmount(message.FromAddress)
	if err != nil {
		return err
	}
	// subtract that amount from the sender
	fromAccountAmount.Sub(fromAccountAmount, amount)
	// if they go negative, they don't have sufficient funds
	// NOTE: we don't use the u.SubtractAccountAmount() function because Utility needs to do this check
	if fromAccountAmount.Sign() == -1 {
		return typesUtil.ErrInsufficientAmount(hex.EncodeToString(message.FromAddress))
	}
	// add the amount to the recipient's account
	if err = u.AddAccountAmount(message.ToAddress, amount); err != nil {
		return err
	}
	// set the sender's account amount
	if err = u.SetAccountAmount(message.FromAddress, fromAccountAmount); err != nil {
		return err
	}
	return nil
}

func (u *UtilityContext) HandleStakeMessage(message *typesUtil.MessageStake) typesUtil.Error {
	publicKey, err := u.BytesToPublicKey(message.PublicKey)
	if err != nil {
		return err
	}
	// ensure above minimum stake
	amount, err := u.CheckAboveMinStake(message.ActorType, message.Amount)
	if err != nil {
		return err
	}
	// ensure signer has sufficient funding for the stake
	signerAccountAmount, err := u.GetAccountAmount(message.Signer)
	if err != nil {
		return err
	}
	// calculate new signer account amount
	signerAccountAmount.Sub(signerAccountAmount, amount)
	if signerAccountAmount.Sign() == -1 {
		return typesUtil.ErrInsufficientAmount(hex.EncodeToString(message.Signer))
	}
	// validators don't have chains field
	if err = u.CheckBelowMaxChains(message.ActorType, message.Chains); err != nil {
		return err
	}
	// ensure actor doesn't already exist
	if exists, err := u.GetActorExists(message.ActorType, publicKey.Address()); err != nil || exists {
		if exists {
			return typesUtil.ErrAlreadyExists()
		}
		return err
	}
	// update account amount
	if err = u.SetAccountAmount(message.Signer, signerAccountAmount); err != nil {
		return err
	}
	// move funds from account to pool
	if err = u.AddPoolAmount(coreTypes.Pools_POOLS_APP_STAKE.FriendlyName(), amount); err != nil {
		return err
	}
	var er error
	store := u.Store()
	// insert actor
	switch message.ActorType {
	case coreTypes.ActorType_ACTOR_TYPE_APP:
		maxRelays, err := u.CalculateAppRelays(message.Amount)
		if err != nil {
			return err
		}
		er = store.InsertApp(publicKey.Address(), publicKey.Bytes(), message.OutputAddress, false, int32(typesUtil.StakeStatus_Staked), maxRelays, message.Amount, message.Chains, typesUtil.HeightNotUsed, typesUtil.HeightNotUsed)
	case coreTypes.ActorType_ACTOR_TYPE_FISH:
		er = store.InsertFisherman(publicKey.Address(), publicKey.Bytes(), message.OutputAddress, false, int32(typesUtil.StakeStatus_Staked), message.ServiceUrl, message.Amount, message.Chains, typesUtil.HeightNotUsed, typesUtil.HeightNotUsed)
	case coreTypes.ActorType_ACTOR_TYPE_SERVICENODE:
		er = store.InsertServiceNode(publicKey.Address(), publicKey.Bytes(), message.OutputAddress, false, int32(typesUtil.StakeStatus_Staked), message.ServiceUrl, message.Amount, message.Chains, typesUtil.HeightNotUsed, typesUtil.HeightNotUsed)
	case coreTypes.ActorType_ACTOR_TYPE_VAL:
		er = store.InsertValidator(publicKey.Address(), publicKey.Bytes(), message.OutputAddress, false, int32(typesUtil.StakeStatus_Staked), message.ServiceUrl, message.Amount, typesUtil.HeightNotUsed, typesUtil.HeightNotUsed)
	}
	if er != nil {
		return typesUtil.ErrInsert(er)
	}
	return nil
}

func (u *UtilityContext) HandleEditStakeMessage(message *typesUtil.MessageEditStake) typesUtil.Error {
	// ensure actor exists
	if exists, err := u.GetActorExists(message.ActorType, message.Address); err != nil || !exists {
		if !exists {
			return typesUtil.ErrNotExists()
		}
		return err
	}
	currentStakeAmount, err := u.GetStakeAmount(message.ActorType, message.Address)
	if err != nil {
		return err
	}
	amount, err := typesUtil.StringToBigInt(message.Amount)
	if err != nil {
		return err
	}
	// ensure new stake >= current stake
	amount.Sub(amount, currentStakeAmount)
	if amount.Sign() == -1 {
		return typesUtil.ErrStakeLess()
	}
	// ensure signer has sufficient funding for the stake
	signerAccountAmount, err := u.GetAccountAmount(message.Signer)
	if err != nil {
		return err
	}
	signerAccountAmount.Sub(signerAccountAmount, amount)
	if signerAccountAmount.Sign() == -1 {
		return typesUtil.ErrInsufficientAmount(hex.EncodeToString(message.Signer))
	}
	if err = u.CheckBelowMaxChains(message.ActorType, message.Chains); err != nil {
		return err
	}
	// update account amount
	if err := u.SetAccountAmount(message.Signer, signerAccountAmount); err != nil {
		return err
	}
	// move funds from account to pool
	if err := u.AddPoolAmount(coreTypes.Pools_POOLS_APP_STAKE.FriendlyName(), amount); err != nil {
		return err
	}
	store := u.Store()
	var er error
	switch message.ActorType {
	case coreTypes.ActorType_ACTOR_TYPE_APP:
		maxRelays, err := u.CalculateAppRelays(message.Amount)
		if err != nil {
			return err
		}
		er = store.UpdateApp(message.Address, maxRelays, message.Amount, message.Chains)
	case coreTypes.ActorType_ACTOR_TYPE_FISH:
		er = store.UpdateFisherman(message.Address, message.ServiceUrl, message.Amount, message.Chains)
	case coreTypes.ActorType_ACTOR_TYPE_SERVICENODE:
		er = store.UpdateServiceNode(message.Address, message.ServiceUrl, message.Amount, message.Chains)
	case coreTypes.ActorType_ACTOR_TYPE_VAL:
		er = store.UpdateValidator(message.Address, message.ServiceUrl, message.Amount)
	}
	if er != nil {
		return typesUtil.ErrInsert(er)
	}
	return nil
}

func (u *UtilityContext) HandleUnstakeMessage(message *typesUtil.MessageUnstake) typesUtil.Error {
	if status, err := u.GetActorStatus(message.ActorType, message.Address); err != nil || status != int32(typesUtil.StakeStatus_Staked) {
		if status != int32(typesUtil.StakeStatus_Staked) {
			return typesUtil.ErrInvalidStatus(status, int32(typesUtil.StakeStatus_Staked))
		}
		return err
	}
	unstakingHeight, err := u.GetUnstakingHeight(message.ActorType)
	if err != nil {
		return err
	}
	if err = u.SetActorUnstaking(message.ActorType, unstakingHeight, message.Address); err != nil {
		return err
	}
	return nil
}

func (u *UtilityContext) HandleUnpauseMessage(message *typesUtil.MessageUnpause) typesUtil.Error {
	pausedHeight, err := u.GetPauseHeight(message.ActorType, message.Address)
	if err != nil {
		return err
	}
	if pausedHeight == typesUtil.HeightNotUsed {
		return typesUtil.ErrNotPaused()
	}
	minPauseBlocks, err := u.GetMinimumPauseBlocks(message.ActorType)
	if err != nil {
		return err
	}
	latestHeight, err := u.GetLatestBlockHeight()
	if err != nil {
		return err
	}
	if latestHeight < int64(minPauseBlocks)+pausedHeight {
		return typesUtil.ErrNotReadyToUnpause()
	}
	if err = u.SetActorPauseHeight(message.ActorType, message.Address, typesUtil.HeightNotUsed); err != nil {
		return err
	}
	return nil
}

func (u *UtilityContext) HandleMessageDoubleSign(message *typesUtil.MessageDoubleSign) typesUtil.Error {
	latestHeight, err := u.GetLatestBlockHeight()
	if err != nil {
		return err
	}
	evidenceAge := latestHeight - message.VoteA.Height
	maxEvidenceAge, err := u.GetMaxEvidenceAgeInBlocks()
	if err != nil {
		return err
	}
	if evidenceAge > int64(maxEvidenceAge) {
		return typesUtil.ErrMaxEvidenceAge()
	}
	pk, er := crypto.NewPublicKeyFromBytes(message.VoteB.PublicKey)
	if er != nil {
		return typesUtil.ErrNewPublicKeyFromBytes(er)
	}
	doubleSigner := pk.Address()
	// burn validator for double signing blocks
	burnPercentage, err := u.GetDoubleSignBurnPercentage()
	if err != nil {
		return err
	}
	if err := u.BurnActor(coreTypes.ActorType_ACTOR_TYPE_VAL, burnPercentage, doubleSigner); err != nil {
		return err
	}
	return nil
}

func (u *UtilityContext) HandleMessageChangeParameter(message *typesUtil.MessageChangeParameter) typesUtil.Error {
	cdc := u.Codec()
	v, err := cdc.FromAny(message.ParameterValue)
	if err != nil {
		return typesUtil.ErrProtoFromAny(err)
	}
	return u.UpdateParam(message.ParameterKey, v)
}

func (u *UtilityContext) GetSignerCandidates(msg typesUtil.Message) ([][]byte, typesUtil.Error) {
	switch x := msg.(type) {
	case *typesUtil.MessageDoubleSign:
		return u.GetMessageDoubleSignSignerCandidates(x)
	case *typesUtil.MessageSend:
		return u.GetMessageSendSignerCandidates(x)
	case *typesUtil.MessageStake:
		return u.GetMessageStakeSignerCandidates(x)
	case *typesUtil.MessageUnstake:
		return u.GetMessageUnstakeSignerCandidates(x)
	case *typesUtil.MessageUnpause:
		return u.GetMessageUnpauseSignerCandidates(x)
	case *typesUtil.MessageChangeParameter:
		return u.GetMessageChangeParameterSignerCandidates(x)
	default:
		return nil, typesUtil.ErrUnknownMessage(x)
	}
}

func (u *UtilityContext) GetMessageStakeSignerCandidates(msg *typesUtil.MessageStake) ([][]byte, typesUtil.Error) {
	pk, er := crypto.NewPublicKeyFromBytes(msg.PublicKey)
	if er != nil {
		return nil, typesUtil.ErrNewPublicKeyFromBytes(er)
	}
	candidates := make([][]byte, 0)
	candidates = append(candidates, msg.OutputAddress)
	candidates = append(candidates, pk.Address())
	return candidates, nil
}

func (u *UtilityContext) GetMessageEditStakeSignerCandidates(msg *typesUtil.MessageEditStake) ([][]byte, typesUtil.Error) {
	output, err := u.GetActorOutputAddress(msg.ActorType, msg.Address)
	if err != nil {
		return nil, err
	}
	candidates := make([][]byte, 0)
	candidates = append(candidates, output)
	candidates = append(candidates, msg.Address)
	return candidates, nil
}

func (u *UtilityContext) GetMessageUnstakeSignerCandidates(msg *typesUtil.MessageUnstake) ([][]byte, typesUtil.Error) {
	output, err := u.GetActorOutputAddress(msg.ActorType, msg.Address)
	if err != nil {
		return nil, err
	}
	candidates := make([][]byte, 0)
	candidates = append(candidates, output)
	candidates = append(candidates, msg.Address)
	return candidates, nil
}

func (u *UtilityContext) GetMessageUnpauseSignerCandidates(msg *typesUtil.MessageUnpause) ([][]byte, typesUtil.Error) {
	output, err := u.GetActorOutputAddress(msg.ActorType, msg.Address)
	if err != nil {
		return nil, err
	}
	candidates := make([][]byte, 0)
	candidates = append(candidates, output)
	candidates = append(candidates, msg.Address)
	return candidates, nil
}

func (u *UtilityContext) GetMessageSendSignerCandidates(msg *typesUtil.MessageSend) ([][]byte, typesUtil.Error) {
	return [][]byte{msg.FromAddress}, nil
}

func (u *UtilityContext) GetMessageDoubleSignSignerCandidates(msg *typesUtil.MessageDoubleSign) ([][]byte, typesUtil.Error) {
	return [][]byte{msg.ReporterAddress}, nil
}
