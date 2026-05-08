import React from 'react'
import ReactDOM from 'react-dom/client'
import { createRouter, createRootRoute, createRoute, RouterProvider } from '@tanstack/react-router'
import { App } from './App'
import { IndexPage } from './routes/index'
import { LoginPage } from './routes/login'
import { SetupPage } from './routes/setup'
import { SettingsRoute } from './routes/settings'
import { ClustersIndex } from './routes/clusters.index'
import { ClustersNew } from './routes/clusters.new'
import { ClusterOverview } from './routes/clusters/$clusterId'
import { ClusterInitialize } from './routes/clusters/$clusterId.initialize'
import { ClusterDeployments } from './routes/clusters/$clusterId.deployments'
import { ClusterApps } from './routes/clusters/$clusterId.apps'
import { DeploymentLogs } from './routes/deployments/$deploymentId'
import { InstallView } from './routes/apps/installs.$installId'
import { Activity } from './routes/activity'
import './index.css'

const root = createRootRoute({ component: App })
const indexR = createRoute({ getParentRoute: () => root, path: '/', component: IndexPage })
const setupR = createRoute({ getParentRoute: () => root, path: '/setup', component: SetupPage })
const loginR = createRoute({ getParentRoute: () => root, path: '/login', component: LoginPage })
const settingsR = createRoute({ getParentRoute: () => root, path: '/settings', component: SettingsRoute })
const clustersIndexR = createRoute({ getParentRoute: () => root, path: '/clusters', component: ClustersIndex })
const clustersNewR = createRoute({ getParentRoute: () => root, path: '/clusters/new', component: ClustersNew })
const clusterR = createRoute({ getParentRoute: () => root, path: '/clusters/$clusterId', component: ClusterOverview })
const clusterInitR = createRoute({ getParentRoute: () => root, path: '/clusters/$clusterId/initialize', component: ClusterInitialize })
const clusterDeploymentsR = createRoute({ getParentRoute: () => root, path: '/clusters/$clusterId/deployments', component: ClusterDeployments })
const clusterAppsR = createRoute({ getParentRoute: () => root, path: '/clusters/$clusterId/apps', component: ClusterApps })
const deploymentR = createRoute({ getParentRoute: () => root, path: '/deployments/$deploymentId', component: DeploymentLogs })
const installViewR = createRoute({ getParentRoute: () => root, path: '/apps/installs/$installId', component: InstallView })
const activityR = createRoute({ getParentRoute: () => root, path: '/activity', component: Activity })
const tree = root.addChildren([indexR, setupR, loginR, settingsR, clustersIndexR, clustersNewR, clusterR, clusterInitR, clusterDeploymentsR, clusterAppsR, deploymentR, installViewR, activityR])
const router = createRouter({ routeTree: tree })

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
)
