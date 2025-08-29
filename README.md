# Reece Iriye DevOps Portfolio

Here is my portfolio. I am deloying this on a Google Cloud Platform VM using Docker and on-prem on a 2-Node Raspberry Pi homelab using Kubernetes.

## HTMX Portfolio Server

### Set-Up

Here are the following set-up's that need to take place when deploying my portfolio on-prem in a Kubernetes cluster and using Docker.

#### Application Local Testing

First, set up a `.env` file using the variables from the `.env.example` file. All these values are required for successfully configuring an SMTP server using your choice of service. I tested these configurations using a Brevo SMTP server.

If you just want to test the application itself and not the rest of the components, the following commands:

```bash
cd htmx
docker buildx build --platform linux/amd64 -t portfolio-server --load .
docker run --env-file .env -p 8080:8080 portfolio-server
```

From here, you can fetch data from the HTMX/Go server using `http://localhost:8080`.

I also set up `air` for this Go application for local testing, so if you would prefer, install the `air` CLI tool and start up the server. This can also be accessed on `http://localhost:8080`.

```bash
cd htmx

go mod tidy && go mod download
go install github.com/air-verse/air@latest

air
```

Now, all code changes will be reflected in the development server immediately.


## Deploy Docker Compose Production Environment on GCP Virtual Machine



```bash
sudo adduser --disabled-password --gecos "" githubactions-deploy-user
sudo usermod -aG docker githubactions-deploy-user
```


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

### Create GCP Service Account

Create a service account in your [Google Cloud Console](https://console.cloud.google.com/). Navigate to **IAM & Admin -> Service Accounts**. 

![Google Cloud Service Account](https://raw.githubusercontent.com/reece-iriye/portfolio-site/main/assets/root-readme/gcloud-service-account.png)

Click **Create Service Account** and name it `githubactions-deploy-user`. Assign the `IAP-Secured Tunnel User` role to the newly created user. Go to the **Keys** tab, then **Add Key -> Create new key -> JSON**. Download the JSON key file, which will be needed for Github Actions secrets. The reason why this needs to be done is to allow these GitHub Actions steps in the `deploy-gcp-vm` job to work:

```yaml
- name: Update Git Repository in VM over IAP
  run: |
    gcloud compute ssh githubactions-deploy-user@${{ secrets.GCP_INSTANCE_NAME }} \
      --tunnel-through-iap \
      --project=${{ secrets.GCP_PROJECT_ID }} \
      --zone=${{ secrets.GCP_ZONE }} \
      --command="cd ~/portfolio-site && git pull origin main"

- name: Update Containers on VM over IAP
  run: |
    gcloud compute ssh githubactions-deploy-user@${{ secrets.GCP_INSTANCE_NAME }} \
      --tunnel-through-iap \
      --project=${{ secrets.GCP_PROJECT_ID }} \
      --zone=${{ secrets.GCP_ZONE }} \
      --command="cd ~/portfolio-site && docker compose pull && docker compose -f compose.prod.yaml up -d"
```

Now, the following secrets need to be added to the GitHub repository. Click on the **Settings** tab, then **Secrets and Variables -> Actions**, then configure the following secrets:
- `GCP_SA_KEY` -> the JSON file contents you downloaded
- `GCP_PROJECT_ID` -> the GCP Project ID (e.g. `devops-portfolio-123456`)
- `GCP_ZONE` -> the zone the VM is located in (e.g. `us-eastern1-a`)
- `GCP_INSTANCE_NAME` -> the name of the VM (e.g. `portfolio-vm`)

In order for this account to have access to the VM, you need to configure the account using your default GCP user that you used previously to SSH into the VM.

```bash
sudo adduser --disabled-password --gecos "" githubactions-deploy-user

sudo -u githubactions-deploy-user bash
cd ~
git clone https://github.com/reece-iriye/portfolio-site portfolio-site  # Or use forked repository if you forked it
cd ./portfolio-site
git branch --set-upstream-to=origin/main main

exit
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

To ensure that your user and `githubactions-deploy-user` can communicate with the Docker daeomon through this Unix socket `/var/run/docker.sock` owned by `root` with permissions `660`, you must run the following command:

```bash
sudo usermod -aG docker $USER
sudo usermod -aG docker githubactions-deploy-user
```

Then, log out and back in, and test `docker run hello-world` and `docker ps` on both accounts to test that it's working.


## Set Up Continuous Deployment

### Enviromnent Variable File Injection via GitHub Secrets

All environment variables that the application uses need to be configured. Reference all existing `.env.example` files, featured in both the `./grafana/` and `./htmx/` directories, and configure all of them in the same GitHub Actions secrets section as before.

![GitHub Secrets Page](https://raw.githubusercontent.com/reece-iriye/portfolio-site/main/assets/root-readme/github-secrets.png)

From here, continuous deployment onto the GCP VM via the `githubactions-deploy-user` service account is almost successfully configured. The `deploy-gcp-vm` job will be invoked upon pushing to main in this repository, or it can be run manually for repository owners. 



