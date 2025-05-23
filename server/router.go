package server

import "nightcord-server/server/routes"

func InitRoute() error {
	routes.InitLanguageRoutes(ginServer)
	routes.InitExecutorRoutes(ginServer)
	routes.InitStorageRoutes(ginServer)
	return nil
}
