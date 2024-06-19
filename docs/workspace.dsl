workspace {

    model {

        # Enable nested groups:    "" "Existing"
        properties {    "" "Existing"
            "structurizr.groupSeparator" "/"
        }    
        # People and software systems
        up4w = softwareSystem "UP4W " {
            up4wagent = container "UP4W Windows Agent" {
                configModule = component "Configuration Module"
                landscapeProService = component "Landscape Pro Service" 
            }
            up4wgui = container "UP4W Windows GUI"
        }
        wslProService = softwareSystem "WSL Pro Service"

        # group "Ubuntu Pro offering for Ubuntu WSL" {
        canonical = person "Canonical Support Line" "24/7 enterprise support for Ubuntu and your full open source stack" "Existing"
        landscape = softwareSystem "Landscape" "Included with your Ubuntu Pro subscription. Automates security patching, auditing, access management and compliance tasks across your Ubuntu estate." "Existing, Service"
        upc = softwareSystem "Ubuntu Pro" "Automates the enablement of Ubuntu Pro services" "Existing"
        # esm = softwareSystem "Canonical ESM" "1,800+ additional high and critical patches, 10 years of maintenance for the whole stack" "Existing"
        # cis = softwareSystem "CIS" "Automates compliance and auditing with the Center for Internet Security (CIS) benchmarks" "Existing"
        # }
        landscapeClient = softwareSystem "Landscape Client" "" "Existing"
        landscapeServer = softwareSystem "Landscape Server" "" "Existing"
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
        up4w -> canonical "Enables 24/7 optional enterprise-grade support from"
        up4w -> landscape "Registers your Windows host and Ubuntu WSL instances with"
        up4w -> ups "Adds your Ubuntu WSL instances to your"
        up4w -> upc "Attaches to Ubuntu Pro"
        up4wgui -> up4wagent "Writes configuration to"
        user -> up4w "Uses"
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


        deploymentEnvironment "UP4WTutorial" {
            g1 = deploymentGroup "1"
            g2 = deploymentGroup "2"
            g3 = deploymentGroup "3"
            g4 = deploymentGroup "4"
            
        
        
            deploymentNode "Your computer = Windows host" "" "Windows" {
                softwareSystemInstance windowsRegistry g1            
            
                deploymentNode "Ubuntu WSL instance (Ubuntu 22.04 LTS)" {
                    softwareSystemInstance landscapeServer g1,g2
                }

                group "UP4W appx" {
                    containerInstance up4wgui g1
                    containerInstance up4wagent g1,g2
                }
                
                deploymentNode "Ubuntu WSL instance (Ubuntu Preview; created locally)" {
                    softwareSystemInstance wslProService g1
                    softwareSystemInstance proClient g1
                    softwareSystemInstance landscapeClient g1
                    
                }
            
                deploymentNode "Ubuntu WSL instance (Ubuntu; created remotely via Landscape Server)" {
                    softwareSystemInstance wslProService g2
                    softwareSystemInstance proClient g2
                    softwareSystemInstance landscapeClient g2
                    
                }            
            
            }
        }


        
        deploymentEnvironment "UbuntuProForWSL" {
            deploymentNode "Your computer = Windows host" "" "Windows" {
                group "UP4W appx" {
                    containerInstance up4wgui
                    containerInstance up4wagent
                }
                softwareSystemInstance windowsRegistry
                # softwareSystemInstance WSL
                deploymentNode "WSL (VM)" "" "" {
                    deploymentNode "Ubuntu WSL instance (distro)" "" "Ubuntu (Preview)" {
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
        
    }

    views {
        systemLandscape "SystemLandscape" {
            include user up4w landscape ups
            autoLayout
        }    
        systemContext up4w "SystemContext" {
            include user up4w landscape ups
            autoLayout
        }
        
        container up4w "SystemContainers" {
            include *
            autoLayout
        }        
        component up4wagent "ContainerComponents" {
            include *
            autoLayout
        }    

        deployment up4w "UbuntuProForWSL" "UbuntuProForWSL" {
            include *
            autoLayout
            properties {
                "structurizr.groups" true
            }            
        }     
        
        deployment up4w "UP4WTutorial" "UP4WTutorial" {
            include *
            # autoLayout
            properties {
                "structurizr.groups" true
            }            
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
            element "Person" {
                metadata false
                shape person
                # background red
                color #ffffff
            }
            element "Existing" {
                background #999999
                color #ffffff
            }
            
            element "Service" {
                background #999999
                color #ffffff
                shape Box
            }            
        }    
    }
    
}
