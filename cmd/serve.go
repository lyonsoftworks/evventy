package cmd

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/lyonsoftworks/evvent/gen/api/v1/events"
	"github.com/lyonsoftworks/evvent/transport"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Configure zap logger.
		logger, err := zap.NewDevelopment()
		if err != nil {
			log.Fatalf("failed to create logger: %v", err)
		}
		defer logger.Sync() // Flushes buffer, if any.
		zap.ReplaceGlobals(logger)

		// Read the configuration from config.yaml using viper.
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		if err := viper.ReadInConfig(); err != nil {
			zap.S().Fatalf("failed to read config file: %v", err)
		}

		// Get the gRPC port and gRPC gateway port values from the configuration.
		grpcPort := viper.GetInt("port")
		grpcGatewayPort := viper.GetInt("gateway_port")
		metricsPort := viper.GetInt("metrics_port")

		// Start the gRPC server.
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
		if err != nil {
			zap.S().Fatalf("failed to listen: %v", err)
		}

		grpcServer := grpc.NewServer()
		transport.Initialize()
		transport.RegisterPrometheusHandler()

		go func() {
			logger.Info("Started metrics", zap.Int16("port", int16(metricsPort)))
			err := http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil)
			if err != nil {
				zap.S().Errorf("Failed to start metrics %s")
			}
		}()

		srv := transport.NewEventServer()
		events.RegisterEventServiceServer(grpcServer, srv)
		reflection.Register(grpcServer)

		zap.S().Infof("gRPC server started on port %d", grpcPort)
		go func() {
			if err := grpcServer.Serve(lis); err != nil {
				zap.S().Fatalf("failed to serve gRPC: %v", err)
			}
		}()

		// Start the gRPC gateway server.
		mux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithInsecure()}

		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		err = events.RegisterEventServiceHandlerFromEndpoint(ctx, mux, fmt.Sprintf("localhost:%d", grpcPort), opts)
		if err != nil {
			zap.S().Fatalf("failed to start gRPC gateway: %v", err)
		}

		zap.S().Infof("gRPC gateway server started on port %d", grpcGatewayPort)
		gwServer := &http.Server{
			Addr:    fmt.Sprintf(":%d", grpcGatewayPort),
			Handler: mux,
		}

		if err := gwServer.ListenAndServe(); err != nil {
			zap.S().Fatalf("failed to start gRPC gateway server: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

}
