# Reece Iriye DevOps Portfolio

Here is my portfolio. I am deloying this on both AWS using ECS/Docker-Compose and on-prem on a homelab using Kubernetes.

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

### Accessing Observability Tools

```bash
ssh -L 3000:localhost:3000 -L 9090:localhost:9090 user@gcp-vmname
```

