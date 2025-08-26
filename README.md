# Reece Iriye DevOps Portfolio

Here is my portfolio. I am deloying this on a Google Cloud Platform VM using Docker and on-prem on a 2-Node Raspberry Pi homelab using Kubernetes.

## HTMX Portfolio Server

### Set-Up

Here are the following set-up's that need to take place when deploying my portfolio on-prem in a Kubernetes cluster and using Docker.

#### Docker Local Testing

First, set up a `.env` file using the variables from the `.env.example` file. All these values are required for successfully configuring an SMTP server using your choice of service. I tested these configurations using a Brevo SMTP server.

Run the following commands:

```bash
cd htmx
docker buildx build --platform linux/amd64 -t portfolio-server --load .
docker run --env-file .env -p 8080:8080 portfolio-server
```

From here, you can fetch data from the HTMX/Go server using `http://localhost:8080`.




## Deploy Docker Compose Production Environment on GCP Virtual Machine


### Create VM

Create a Google Cloud Platform VM on the [Google Cloud console](https://console.cloud.google.com). Specify all resource requirements when creating the VM. Because all containers for my portfolio application consume minimal resources, I opted for the smallest resource count to save money and resources, while being enough to run Docker containers and the `dockerd` daemon.

![Cloud Platform Home Page](https://raw.githubusercontent.com/reece-iriye/portfolio-site/main/assets/root-readme/gcloud-homepage-create-vm-button.png)

### Access VM Using SSH

```bash
gcloud init  # Follow log-in instructions
gcloud compute ssh --zone <ZONE> <INSTANCE_NAME> --project <PROJECT_NAME>
```

### Install Git on VM


```bash
sudo apt-get update
sudo apt-get install git
git --version
```

### Add Docker to VM

I followed the instructions directly from the Docker docs for installing Docker onto a Debian Bookworm 12 VM, which can be found [here](https://docs.docker.com/engine/install/debian/#install-using-the-repository).

```bash
for pkg in docker.io docker-doc docker-compose podman-docker containerd runc; do sudo apt-get remove $pkg; done

# Add Docker's official GPG key:
sudo apt-get update
sudo apt-get install ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add the repository to Apt sources:
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

# Install desired Docker plugins and containerd
sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Test Docker Image Pull functionality
sudo docker run hello-world
```

To ensure that your user can communicate with the Docker daeomon through this Unix socket `/var/run/docker.sock` owned by `root` with permissions `660`, you must run the following command:

```bash
sudo usermod -aG docker $USER
```

Then, log out and back in, and test `docker ps` to test that it's working.



## Set Up Continuous Deployment



### Configure Deployment User on VM

Create the Linux user. I decided to create the user inside the `/srv/github` directory to separate CI/CD service accounts associated with Github Actions from the rest of the users in `/home`.

```bash
cd /srv
sudo mkdir -p ./github
cd ./github

sudo useradd -m -r -d /srv/github/deployuser -s /bin/bash deployuser
```

Then, add `deployuser` to Docker group.

```bash
sudo usermod -aG docker deployuser
```


### Pull GitHub Repository and Configure Environment Variables

Pull the repository's main branch and configure it to successfully.

```bash
sudo su - deployuser
git clone https://github.com/reece-iriye/portfolio-site
cd ./portfolio-site
git branch --set-upstream-to=origin/main main
exit

# If portfolio site is created by separate user, run this command to ensure deployuser owns all files within the directory
sudo chown -R deployuser:deployuser /srv/github/deployuser/portfolio-site
```

### Enviromnent Variable File Injection

All environment varibles need to be configured in each directory. Please reference the `.env.example` files to showcase exactly which variables need to be created, and create them in each `.env` file in each subdirectory in accordance with the `.env.example` files.


### Configure SSH Public Key for GitLab



Now, on a separate shell outside of the GCP VM (I did this on my Mac ZSH shell), generate an SSH key.

```bash
cd ~/.ssh
ssh-keygen -t ed25519 -C "github-actions-deploy" -f github_actions_deploy_key
```

Going back to the GCP VM shell, run these commands to expose `deployuser` to the device's public key.

```bash
# Create SSH directory to store public key
sudo su - deployuser
mkdir -p /srv/github/deployuser/.ssh
chmod 700 /srv/github/deployuser/.ssh

# Paste public key configs into authorized_keys file
vim /srv/github/deployuser/.ssh/authorized_keys
<PASTE_PUBLIC_KEY_CONTENTS>
:wq

# Ensure file permissions are stored
chmod 600 /srv/github/.ssh/authorized_keys

# Ensure ownership is correct
chown -R deployuser:deployuser /srv/github/deployuser/.ssh
```

Check the output of this `ls` command to ensure file permissions are correctly configured:
```bash
ls -ld /srv /srv/github /srv/github/deployuser /srv/github/deployuser/.ssh /srv/github/deployuser/.ssh/authorized_keys
```

Expected output:
```
drwxr-xr-x 3 root       root       ... /srv
drwxr-xr-x 3 root       root       ... /srv/github
drwxr-xr-x 5 deployuser deployuser ... /srv/github/deployuser
drwx------ 2 deployuser deployuser ... /srv/github/deployuser/.ssh
-rw------- 1 deployuser deployuser ... /srv/github/deployuser/.ssh/authorized_keys
```

Finally, test external VM access from the same device that the private key was created on to make sure this can work. On my Mac, this required me running the following command, while using `gcloud compute services list` to grab the external IP address of the Virtual Machine.

```bash
# Test SSH Connection to the VM to ensure it works correctly. Enable verbose logging to fetch errors.
ssh -i ~/.ssh/github_actions_deploy_key deployuser@<VM_EXTERNAL_IP> -vvv
```

From here, continuous deployment onto the GCP VM via the `deployuser` Linux service account is almost successfully configured. The `deploy-gcp-vm` job will be invoked upon pushing to main in this repository. All that needs to be configured now are the secrets on GitHub. The jobs will fail upon each commit to main, because the empty secrets.



### Configure Secrets on GitHub


![GitHub Secrets Page](https://raw.githubusercontent.com/reece-iriye/portfolio-site/assets/root-readme/github-secrets.png)

