#!groovy

import jenkins.model.*
import hudson.security.*
import jenkins.model.JenkinsLocationConfiguration
import hudson.security.csrf.DefaultCrumbIssuer

def instance = Jenkins.getInstance()
println "--> creating local user '{{.User}}'"

def hudsonRealm = new HudsonPrivateSecurityRealm(false)
hudsonRealm.createAccount('{{.User}}', '{{.Password}}')
instance.setSecurityRealm(hudsonRealm)

def strategy = new FullControlOnceLoggedInAuthorizationStrategy()
strategy.setAllowAnonymousRead(false)
instance.setAuthorizationStrategy(strategy)

println "--> setting jenkins url '{{.Url}}'"

jenkinsLocation = JenkinsLocationConfiguration.get()
jenkinsLocation.setUrl('{{.Url}}')
jenkinsLocation.setAdminAddress('{{.AdminEmail}}')
jenkinsLocation.save()

println "--> setting up default crumbs issuer"
instance.setCrumbIssuer(new DefaultCrumbIssuer(true))

println "--> setting up JNLP"
Set<String> agentProtocolsList = ['JNLP4-connect', 'Ping']
instance.setAgentProtocols(agentProtocolsList)
instance.setSlaveAgentPort({{ .AgentPort }})
instance.setNumExecutors({{ .Executors }})
instance.save()




