package module

import (
	"context"
	"encoding/json"

	abci "github.com/cometbft/cometbft/abci/types"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/errors"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	modulev1 "cosmossdk.io/api/cosmos/nft/module/v1"

	"cosmossdk.io/x/nft"
	"cosmossdk.io/x/nft/client/cli"
	"cosmossdk.io/x/nft/keeper"
	"cosmossdk.io/x/nft/simulation"
)

var (
	_ module.AppModule           = AppModule{}
	_ module.AppModuleBasic      = AppModuleBasic{}
	_ module.AppModuleSimulation = AppModule{}
)

// AppModuleBasic defines the basic application module used by the nft module.
type AppModuleBasic struct {
	cdc codec.Codec
	ac  address.Codec
}

// Name returns the nft module's name.
func (AppModuleBasic) Name() string {
	return nft.ModuleName
}

// RegisterServices registers a gRPC query service to respond to the
// module-specific gRPC queries.
func (am AppModule) RegisterServices(registrar grpc.ServiceRegistrar) error {
	nft.RegisterMsgServer(registrar, am.keeper)
	nft.RegisterQueryServer(registrar, am.keeper)
	return nil
}

// RegisterLegacyAminoCodec registers the nft module's types for the given codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {}

// RegisterInterfaces registers the nft module's interface types
func (AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	nft.RegisterInterfaces(registry)
}

// DefaultGenesis returns default genesis state as raw bytes for the nft
// module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(nft.DefaultGenesisState())
}

// ValidateGenesis performs genesis state validation for the nft module.
func (ab AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config sdkclient.TxEncodingConfig, bz json.RawMessage) error {
	var data nft.GenesisState
	if err := cdc.UnmarshalJSON(bz, &data); err != nil {
		return errors.Wrapf(err, "failed to unmarshal %s genesis state", nft.ModuleName)
	}

	return nft.ValidateGenesis(data, ab.ac)
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the nft module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx sdkclient.Context, mux *gwruntime.ServeMux) {
	if err := nft.RegisterQueryHandlerClient(context.Background(), mux, nft.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// GetQueryCmd returns a no-op command for the nft module.
// Queries for NFT are registered by autocli.
func (ab AppModuleBasic) GetQueryCmd() *cobra.Command {
	return nil
}

// GetTxCmd returns the transaction commands for the nft module
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// AppModule implements the sdk.AppModule interface
type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
	// TODO accountKeeper,bankKeeper will be replaced by query service
	accountKeeper nft.AccountKeeper
	bankKeeper    nft.BankKeeper
	registry      cdctypes.InterfaceRegistry
}

// NewAppModule creates a new AppModule object
func NewAppModule(cdc codec.Codec, keeper keeper.Keeper, ak nft.AccountKeeper, bk nft.BankKeeper, registry cdctypes.InterfaceRegistry) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc, ac: ak},
		keeper:         keeper,
		accountKeeper:  ak,
		bankKeeper:     bk,
		registry:       registry,
	}
}

var (
	_ appmodule.AppModule   = AppModule{}
	_ appmodule.HasServices = AppModule{}
)

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (am AppModule) IsOnePerModuleType() {}

// IsAppModule implements the appmodule.AppModule interface.
func (am AppModule) IsAppModule() {}

// Name returns the nft module's name.
func (AppModule) Name() string {
	return nft.ModuleName
}

// InitGenesis performs genesis initialization for the nft module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState nft.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	am.keeper.InitGenesis(ctx, &genesisState)
	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns the exported genesis state as raw bytes for the nft
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(gs)
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// ____________________________________________________________________________

// AppModuleSimulation functions

// GenerateGenesisState creates a randomized GenState of the nft module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	simulation.RandomizedGenState(simState)
}

// RegisterStoreDecoder registers a decoder for nft module's types
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {
	sdr[keeper.StoreKey] = simulation.NewDecodeStore(am.cdc)
}

// WeightedOperations returns the all the nft module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	return simulation.WeightedOperations(
		am.registry,
		simState.AppParams, simState.Cdc, simState.TxConfig,
		am.accountKeeper, am.bankKeeper, am.keeper,
	)
}

//
// App Wiring Setup
//

func init() {
	appmodule.Register(&modulev1.Module{},
		appmodule.Provide(ProvideModule),
	)
}

type NftInputs struct {
	depinject.In

	StoreService store.KVStoreService
	Cdc          codec.Codec
	Registry     cdctypes.InterfaceRegistry

	AccountKeeper nft.AccountKeeper
	BankKeeper    nft.BankKeeper
}

type NftOutputs struct {
	depinject.Out

	NFTKeeper keeper.Keeper
	Module    appmodule.AppModule
}

func ProvideModule(in NftInputs) NftOutputs {
	k := keeper.NewKeeper(in.StoreService, in.Cdc, in.AccountKeeper, in.BankKeeper)
	m := NewAppModule(in.Cdc, k, in.AccountKeeper, in.BankKeeper, in.Registry)

	return NftOutputs{NFTKeeper: k, Module: m}
}
