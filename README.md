# To validate the Private ACR with the help of terratest

### With the help of terratest, you can run the test cases on the private ACR to test the network access of the ACR and perform the docker operations on the private ACR.

### Follow the below steps to run the terratest code:


You need to define these values in your code before running:-

                    tag = "1.0.1"              // you can change tag values as you want
                    expectedPullResponse = true // define to test pull operation
                    expectedPushResponse = true // define to test push operation
                    testImage = "hello-world:latest" // define to test image
                    expectedPublicNetworkAccess = "Disabled" // define to test public network access

Step 1:- Run the go initialization command:

            go mod init < name >

Step 2:- Run the tidy command to install the packages:-

            go mod tidy

Step 3:- Run the test command:-

            go test -v
