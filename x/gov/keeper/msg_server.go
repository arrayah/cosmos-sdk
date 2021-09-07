package keeper

import (
	"context"
	"fmt"
	"strconv"

	"github.com/armon/go-metrics"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/gov/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the gov MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (k msgServer) SubmitProposal(goCtx context.Context, msg *types.MsgSubmitProposal) (*types.MsgSubmitProposalResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	proposal, err := k.Keeper.SubmitProposal(ctx, msg.GetContent())
	if err != nil {
		return nil, err
	}

	defer telemetry.IncrCounter(1, types.ModuleName, "proposal")

	votingStarted, err := k.Keeper.AddDeposit(ctx, proposal.ProposalId, msg.GetProposer(), msg.GetInitialDeposit())
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.GetProposer().String()),
		),
	)

	submitEvent := sdk.NewEvent(types.EventTypeSubmitProposal, sdk.NewAttribute(types.AttributeKeyProposalType, msg.GetContent().ProposalType()))
	if votingStarted {
		submitEvent = submitEvent.AppendAttributes(
			sdk.NewAttribute(types.AttributeKeyVotingPeriodStart, fmt.Sprintf("%d", proposal.ProposalId)),
		)
	}

	ctx.EventManager().EmitEvent(submitEvent)
	return &types.MsgSubmitProposalResponse{
		ProposalId: proposal.ProposalId,
	}, nil
}

func (k msgServer) SubmitProposal2(goCtx context.Context, msg *types.MsgSubmitProposal2) (*types.MsgSubmitProposal2Response, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	msgs, err := msg.GetMessages()
	if err != nil {
		return nil, err
	}

	proposal, err := k.Keeper.SubmitProposal2(ctx, msgs)
	if err != nil {
		return nil, err
	}

	defer telemetry.IncrCounter(1, types.ModuleName, "proposal")

	votingStarted, err := k.Keeper.AddDeposit2(ctx, proposal.ProposalId, msg.GetProposer(), msg.GetInitialDeposit())
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.GetProposer().String()),
		),
	)

	submitEvent := sdk.NewEvent(types.EventTypeSubmitProposal)
	if votingStarted {
		submitEvent = submitEvent.AppendAttributes(
			sdk.NewAttribute(types.AttributeKeyVotingPeriodStart, fmt.Sprintf("%d", proposal.ProposalId)),
		)
	}

	ctx.EventManager().EmitEvent(submitEvent)
	return &types.MsgSubmitProposal2Response{
		ProposalId: proposal.ProposalId,
	}, nil
}

func (k msgServer) Vote(goCtx context.Context, msg *types.MsgVote) (*types.MsgVoteResponse, error) {
	var err error
	ctx := sdk.UnwrapSDKContext(goCtx)
	accAddr, accErr := sdk.AccAddressFromBech32(msg.Voter)
	if accErr != nil {
		return nil, accErr
	}

	version := k.Keeper.GetProposalVersion(ctx, msg.ProposalId)
	switch version {
	case types.Version1:
		err = k.Keeper.AddVote(ctx, msg.ProposalId, accAddr, types.NewNonSplitVoteOption(msg.Option))
	case types.Version2:
		err = k.Keeper.AddVote2(ctx, msg.ProposalId, accAddr, types.NewNonSplitVoteOption(msg.Option))
	default:
		err = sdkerrors.Wrapf(types.ErrUnknownProposal, "%d", msg.ProposalId)
	}
	if err != nil {
		return nil, err
	}

	defer telemetry.IncrCounterWithLabels(
		[]string{types.ModuleName, "vote"},
		1,
		[]metrics.Label{
			telemetry.NewLabel("proposal_id", strconv.Itoa(int(msg.ProposalId))),
		},
	)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Voter),
		),
	)

	return &types.MsgVoteResponse{}, nil
}

func (k msgServer) VoteWeighted(goCtx context.Context, msg *types.MsgVoteWeighted) (*types.MsgVoteWeightedResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	accAddr, accErr := sdk.AccAddressFromBech32(msg.Voter)
	if accErr != nil {
		return nil, accErr
	}
	err := k.Keeper.AddVote(ctx, msg.ProposalId, accAddr, msg.Options)
	if err != nil {
		return nil, err
	}

	defer telemetry.IncrCounterWithLabels(
		[]string{types.ModuleName, "vote"},
		1,
		[]metrics.Label{
			telemetry.NewLabel("proposal_id", strconv.Itoa(int(msg.ProposalId))),
		},
	)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Voter),
		),
	)

	return &types.MsgVoteWeightedResponse{}, nil
}

func (k msgServer) Deposit(goCtx context.Context, msg *types.MsgDeposit) (*types.MsgDepositResponse, error) {
	var (
		err           error
		votingStarted bool
	)
	ctx := sdk.UnwrapSDKContext(goCtx)
	accAddr, err := sdk.AccAddressFromBech32(msg.Depositor)
	if err != nil {
		return nil, err
	}

	version := k.Keeper.GetProposalVersion(ctx, msg.ProposalId)
	switch version {
	case types.Version1:
		votingStarted, err = k.Keeper.AddDeposit(ctx, msg.ProposalId, accAddr, msg.Amount)
	case types.Version2:
		votingStarted, err = k.Keeper.AddDeposit2(ctx, msg.ProposalId, accAddr, msg.Amount)
	default:
		err = sdkerrors.Wrapf(types.ErrUnknownProposal, "%d", msg.ProposalId)
	}
	if err != nil {
		return nil, err
	}

	defer telemetry.IncrCounterWithLabels(
		[]string{types.ModuleName, "deposit"},
		1,
		[]metrics.Label{
			telemetry.NewLabel("proposal_id", strconv.Itoa(int(msg.ProposalId))),
		},
	)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Depositor),
		),
	)

	if votingStarted {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeProposalDeposit,
				sdk.NewAttribute(types.AttributeKeyVotingPeriodStart, fmt.Sprintf("%d", msg.ProposalId)),
			),
		)
	}

	return &types.MsgDepositResponse{}, nil
}
