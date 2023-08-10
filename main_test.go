package test
import (
    "testing"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "os/exec"
	"os"
    "strings"
    "github.com/stretchr/testify/assert"
    "github.com/gruntwork-io/terratest/modules/terraform"

)

var (
    tag = "1.0.1"
    expectedPullResponse = true
    expectedPushResponse = true
    testImage = "hello-world:latest"
    expectedPublicNetworkAccess = "Disabled"
    

)

func TestTerraformOutputs(t *testing.T) {
    t.Parallel()
    // Path to ACR terraform Module and terratest.tfvars file 
    terraform1Options := &terraform.Options{
        TerraformDir: "../module",
        VarFiles:    []string{"../test/terraform.tfvars"},
    }

    defer terraform.Destroy(t, terraform1Options)
    terraform.InitAndApply(t, terraform1Options)
	
    // fetch value from the ENV variable
	subscriptionID := os.Getenv("TF_VAR_azure_subscription_id")

    // fetch values from output.tf file
    acrName := terraform.Output(t, terraform1Options, "acr_name")
    resourceGroupName := terraform.Output(t, terraform1Options, "resource_group_name")
    acrURL := terraform.Output(t, terraform1Options, "login_server")
    password := terraform.Output(t, terraform1Options, "acr_admin_password")

    // fetch the access token using subscription 
    accessToken, err := getAccessToken(subscriptionID)
    if err != nil {
        t.Errorf("Failed to get access token: %s", err.Error())
        return
    }

    // fetching the ACR details 
    acrDetails, err := getACRDetails(accessToken, subscriptionID, resourceGroupName, acrName)
    assert.NoError(t, err)

    // Calling function to validate that ACR is private or not
    publicNetworkAccessFirst := isACRPrivate(acrDetails)
   

    // Create ACR image name 
    acrImageName := fmt.Sprintf("%s/hello-world:%s", acrURL, tag)
    fmt.Println("ACR image:-", acrImageName)

    // Tag the test image with ACR image name
    taggedImage := TagImageWithACR(testImage,acrImageName)

    // Docker login for the ACR
    err = DockerLogin(acrName, password, acrURL)
    if err != nil {
        fmt.Println("Error in docker login!!:", err)
    }
    
    // Push the image to ACR
    actullPushResponse, err := PushImageToACR(taggedImage)
    if err != nil {
        fmt.Println("Error:", err)
        return
    }

    if actullPushResponse {
        fmt.Println("Image pushed to ACR successfully.")
    } else {
        fmt.Println("Failed to push image to ACR.")
    }

    err = DeleteAllDockerImages()
    if err != nil {
        fmt.Println("Error:", err)
    }
   
    // Pull the image from the ACR
    actullPullResponse, err := PullTestImage(taggedImage)
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    
    if actullPullResponse {
        fmt.Println("Pull operation was successful.")
    } else {
        fmt.Println("Pull operation failed.")
    }


    // test cases....
    
    // First test case - To check the access of the ACR [ Private or Public ]
    t.Run(fmt.Sprintf("Checking the Access for this ACR: %s", acrName), func(t *testing.T) {
        assert.Equal(t, expectedPublicNetworkAccess, publicNetworkAccessFirst, "publicNetworkAccess mismatch")
    })

    // Second test case - To validate the image push operation to the ACR    
    t.Run(fmt.Sprintf("Validate the images pushed to ACR : %s", acrImageName), func(t *testing.T) {
        assert.Equal(t, expectedPushResponse, actullPushResponse, "Image is not pushed from ACR")
    })
    
    // Second test case - To validate the image pull operation to the ACR    
    t.Run(fmt.Sprintf("Validate the pulled images from ACR : %s", acrImageName), func(t *testing.T) {
        assert.Equal(t, expectedPullResponse, actullPullResponse, "Image is not pull from ACR")

    })

 }



// To fetch the access token for the REST API.
func getAccessToken(subscriptionID string) (string, error) {
    cmd := exec.Command("az", "account", "get-access-token", "--query", "accessToken", "--output", "tsv", "--subscription", subscriptionID)
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(output)), nil
}



// To fetch the details of ACR 
func getACRDetails(accessToken, subscriptionID, resourceGroupName, acrName string) (map[string]interface{}, error) {
    url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerRegistry/registries/%s?api-version=2023-06-01-preview", subscriptionID, resourceGroupName, acrName)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+accessToken)
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    var acrDetails map[string]interface{}
    err = json.Unmarshal(body, &acrDetails)
    if err != nil {
        return nil, err
    }
    // acrDetailsJSON, err := json.MarshalIndent(acrDetails, "", "    ")
    // if err != nil {
    //  return nil, err
    // }
    // fmt.Println("ACR Details:")
    // fmt.Println(string(acrDetailsJSON))
    return acrDetails, nil

}


// To fetch the check the network access status
func isACRPrivate(acrDetails map[string]interface{}) string {
    // Check if the "properties" field exists and its value is a map
    if properties, ok := acrDetails["properties"].(map[string]interface{}); ok {
        // Check if the "publicNetworkAccess" property exists and its value is a string
        if publicNetworkAccess, ok := properties["publicNetworkAccess"].(string); ok {
            return publicNetworkAccess
        } else {
            return "publicNetworkAccess not found or not a string"
        }
    } else {
        return "properties not found"
    }
}


// login to the private ACR 
func DockerLogin(acrName, password, acrURL string) error {
    dockerLoginCmd := exec.Command("docker", "login", acrURL, "--username", acrName, "--password", password)
    output, err := dockerLoginCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("error authenticating with Docker: %v\nOutput: %s", err, output)
    }
    fmt.Println("Authenticated with Docker successfully")
    return nil
}


// To pull the image from the Private ACR
func PullTestImage(testImage string) (bool, error) {
    pullTestImage := exec.Command("docker", "pull", testImage)
    err := pullTestImage.Run()
    if err != nil {
        return false, nil
    }
    checkTestImage := exec.Command("docker", "images")
    checkTestImage.Run()

    return true, nil
}



// To tag the image for push operation
func TagImageWithACR(testImage, acrImageName string) string {
    acrTagCmd := exec.Command("docker", "tag", testImage, acrImageName)
    output, err := acrTagCmd.CombinedOutput()
    if err != nil {
        panic(fmt.Errorf("error Tagging image with ACR Repo: %v, Output: %s", err, string(output)))
    }
    fmt.Printf("Image %s Tagged with %s Successfully\n", testImage, acrImageName)
    // Return the acrImageName after tagging
    return acrImageName
}


// To push the image to  Private ACR
func PushImageToACR(acrImageName string) (bool, error) {
    acrPushCmd := exec.Command("docker", "push", acrImageName)
    err := acrPushCmd.Run()
    if err != nil {
        fmt.Printf("Error pushing image to ACR: %v\n", err)
        return false, err
    }

    return true, nil
}


// Delete all the local images before pull
func DeleteAllDockerImages() error {
    cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
    output, err := cmd.Output()
    if err != nil {
        return fmt.Errorf("error running 'docker images' command: %v\nOutput: %s", err, output)
    }
    imageNames := strings.Split(strings.TrimSpace(string(output)), "\n")
    for _, imageName := range imageNames {
        cmd := exec.Command("docker", "rmi", "-f", imageName)
        err := cmd.Run()
        if err != nil {
            return err
        }
        fmt.Println("Deleted image:", imageName)
    }
    return nil
}