workspace {

    model {

        # Enable nested groups:    "" "Existing"
        properties {    "" "Existing"
            "structurizr.groupSeparator" "/"
        }
        
        # Two software systems are defined
        
        # up4w_a is the general software architecture
        up4w_a = softwareSystem "Ubuntu Pro for WSL" {
            up4wagent = container "Ubuntu Pro for WSL Agent" {
                configModule = component "Configuration Module"
                landscapeProService = component "Landscape Pro Service"
            }
            up4wgui = container "UP4W Windows GUI"
        }
        wslProService = softwareSystem "WSL Pro Service"
        
        # up4w_f provides more detail relating to firewall requirements
        up4w_f = softwareSystem "UP4W" {
            up4wagent_f = container "Ubuntu Pro for WSL Agent" {
                configModule_f = component "Configuration Module"
                proAgent_f = component "Windows Pro Agent" 
            }
        }

        # group "Ubuntu Pro offering for Ubuntu WSL" {
        canonical = person "Canonical Support Line" "24/7 enterprise support for Ubuntu and your full open source stack" "Existing"
        landscape = softwareSystem "Landscape" "Included with your Ubuntu Pro subscription. Automates security patching, auditing, access management and compliance tasks across your Ubuntu estate." "Existing, Service"
        upc = softwareSystem "Ubuntu Pro" "Automates the enablement of Ubuntu Pro services" "Existing"
        # esm = softwareSystem "Canonical ESM" "1,800+ additional high and critical patches, 10 years of maintenance for the whole stack" "Existing"
        # cis = softwareSystem "CIS" "Automates compliance and auditing with the Center for Internet Security (CIS) benchmarks" "Existing"
        # }
        landscapeClient = softwareSystem "Landscape Client" "" "Existing"
        landscapeServer = softwareSystem "Landscape Server" "" "Existing"
        # next two added from FireWall src
        contractServer = softwareSystem "Canonical Contract Server" "" "Existing"
        microsoftStore = softwareSystem "Microsoft Store" "" "Existing"
        proClient = softwareSystem "Ubuntu Pro Client" "" "Existing"        
        user = person "User" 
        windowsRegistry = softwareSystem "Windows registry" "" "Existing"
        ups = softwareSystem "Ubuntu Pro subscription" "" "Existing, Service"
        # wsl = softwareSystem "WSL" "" "Existing"         
        
        
        # Relationships
        configModule -> landscapeProService "Configures"
        configModule -> windowsRegistry "Reads configuration from"
        configModule -> wslProService "Sends configuration to"
        landscapeServer -> landscapeClient "Reads from and sends instructions to"
        landscapeServer -> landscapeProService "Reads from and sends instructions to"
        up4w_a -> canonical "Enables 24/7 optional enterprise-grade support from"
        up4w_a -> landscape "Registers your Windows host and Ubuntu WSL instances with"
        up4w_a -> ups "Adds your Ubuntu WSL instances to your"
        up4w_a -> upc "Attaches to Ubuntu Pro"
        up4wgui -> up4wagent "Writes configuration to"
        user -> up4w_a "Uses"
        wslProService -> landscapeClient "Configures"
        wslProService -> proClient "Configures"
        # landscapeClient -> landscapeServer "Distro information
        # landscapeProService -> landscapeClient "Executes commands"
        # landscapeProService -> landscapeServer "Connects to, sends information to, and receives commands from""
        # landscapeProService -> landscapeServer "System and WSL information"
        # landscapeProService -> wsl "Starts and stops Ubuntu WSL instances created by"
        # landscapeServer -> landscapeClient "Distro management commands"
        # landscapeServer -> landscapeProService "Lifetime management commands"
        # upc -> cis "Automates compliance and auditing"
        # upc -> esm "Enables stability, security, and compliance with"
        
        # Additional relationships to support firewall views
        configModule -> proAgent_f "Configures"
        configModule_f -> proAgent_f "Configures"
        wslProService -> proAgent_f   "tcp/grpc(dynamic:49152-65535)\nIP: Hyper-V Virtual Ethernet Adapter Addr"
        proAgent_f -> microsoftStore "tcp/https(443)\nIP: MS Network [1]"
        proAgent_f -> landscapeServer "tcp/grpc(6554)\nIP: On-premises Landscape address"
        landscapeClient -> landscapeServer "tcp/https(443)\nIP: On-premises Landscape address"
        proAgent_f -> contractServer "tcp/https(443)\nIP: contracts.canonical.com"
        proClient -> contractServer  "tcp/https(443)\nIP: contracts.canonical.com"

 deploymentEnvironment "Tutorial" {
            g1 = deploymentGroup "1"
            g2 = deploymentGroup "2"
            g3 = deploymentGroup "3"
            g4 = deploymentGroup "4"
            
        
            deploymentNode "Your computer = Windows host" "" "Windows" {
                softwareSystemInstance windowsRegistry g1            
            
                deploymentNode "Ubuntu WSL instance (Ubuntu 22.04 LTS)" {
                    softwareSystemInstance landscapeServer g1,g2
                }

                group "UP4W app" {
                    containerInstance up4wgui g1
                    containerInstance up4wagent g1,g2
                }
                
                deploymentNode "Ubuntu WSL instance (created locally on Windows host)" {
                    softwareSystemInstance wslProService g1
                    softwareSystemInstance proClient g1
                    softwareSystemInstance landscapeClient g1
                    
                }
            
                deploymentNode "Ubuntu WSL instance (created remotely via Landscape Server)" {
                    softwareSystemInstance wslProService g2
                    softwareSystemInstance proClient g2
                    softwareSystemInstance landscapeClient g2
                    
                }            
            
            }
        }


        
        deploymentEnvironment "Production" {
            deploymentNode "Your computer = Windows host" "" "Windows" {
                group "UP4W app" {
                    containerInstance up4wgui
                    containerInstance up4wagent
                }
                softwareSystemInstance windowsRegistry
                # softwareSystemInstance WSL
                deploymentNode "WSL (VM)" "" "" {
                    deploymentNode "Ubuntu WSL instance" "" "Ubuntu" {
                        softwareSystemInstance wslProService
                        softwareSystemInstance proClient
                        softwareSystemInstance landscapeClient
                    }
                    
                }
            }
            deploymentNode "Someone else's computer" "" "" {
                softwareSystemInstance landscapeServer
            }
        }



        # Additional environments for firewall views
        deploymentEnvironment "Firewall" {
            deploymentNode "Windows host" "" "Windows" {
#                group "Ubuntu Pro for WSL Application" {
                    containerInstance up4wagent_f
#                }
                # softwareSystemInstance WSL
                deploymentNode "WSL (VM)" "" "" {
                    deploymentNode "Ubuntu WSL instance" "" "Ubuntu" {
                        softwareSystemInstance wslProService
                        softwareSystemInstance proClient
                        softwareSystemInstance landscapeClient
                    }
                    
                }
            }
            deploymentNode "Microsoft Network" "" "" {
                softwareSystemInstance microsoftStore
            }

            deploymentNode "Canonical Network" "" "" {
                softwareSystemInstance contractServer
            }

            deploymentNode "On premises Landscape Server" "" "" {
                softwareSystemInstance landscapeServer
            }
        }
        
    }
    

    # general architecture

    views {
        systemLandscape "SystemLandscape" {
            include user up4w_a landscape ups
            autoLayout
        }    
        
        container up4w_a "SystemContainers" {
            include *
            exclude up4w_f
            autoLayout
        }        
        component up4wagent "ContainerComponents" {
            include *
            exclude up4w_f
            autoLayout
        }    
        
        deployment up4w_a "Production" "Production" {
            include *
            exclude "landscapeClient -> landscapeServer"
            autoLayout
            properties {
                "structurizr.groups" true
            }            
        }     
        
        deployment up4w_a "Tutorial" "Tutorial" {
            include *
            exclude "landscapeClient -> landscapeServer"
            autoLayout
            properties {
                "structurizr.groups" true
            }            
        }     
        
        # architecture for firewall reqs
        
        container up4w_f "SystemContainersDeploy" {
            include *
            exclude up4w_a
            autoLayout
        }        
        
        component up4wagent_f "ContainerComponentsDeploy" {
            include *
            exclude up4w_a
            autoLayout
        }
        
        deployment up4w_f "Firewall" "Firewall" {
            include *
            exclude "wslProService -> proClient"
            exclude "wslProService -> landscapeClient"
            exclude "landscapeServer -> landscapeClient"
            autoLayout
            properties {
                "structurizr.groups" true
            }            
        }

        # don't add a title to the generated diagrams
        properties {
            "plantuml.title" false
        }
        
        
        theme default

        styles {
            element "Element" {
                metadata false
                fontSize 25               
                }
            # element "Software System" {
                # metadata false
                # background #1168bd
                # color #ffffff
            # }
            # element "Person" {
                # metadata false
                # shape person
                # background #08427b
                # color #ffffff
            # }
            element "Existing" {
                background #999999
                color #ffffff
            }
            
            element "Service" {
                background #999999
                color #ffffff
                shape Box
            }            
            relationship "Relationship" {
                fontSize 24
            }            
        }    
    }
}
