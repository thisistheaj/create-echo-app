# create-echo-app

## About

An easy way to get started with a new echo project, using [GORM](https://gorm.io/) + [PostgreSQL](https://www.postgresql.org/) for the backend, and [templ](https://templ.dev/) + [tailwindcss](https://tailwindcss.com/) for the frontend, hosted on [Railway](https://railway.app).

## Quickstart

To get started, clone the repository and run the setup script:
```
git clone https://github.com/thisistheaj/create-echo-app.git <your_project_name>
cd <your_project_name>
bash ./scripts/setup.sh
```

Or as one line:
```
git clone https://github.com/thisistheaj/create-echo-app.git <your_project_name> && cd $_ && bash ./scripts/setup.sh
```

## Other Dependencies

You will need air and templ installed for local development, if you don't have them already:

```
go install github.com/air-verse/air@latest
go install github.com/a-h/templ/cmd/templ@latest
```

## Deployment

Your create-echo-app project is ready to deploy to Railway, even before you edit it. 

To be able to deploy your project to Railway, you first need to login to railway and save a [Railway API key](https://docs.railway.app/guides/public-api#creating-a-token):

```
railway login
echo "<your_railway_api_key>" > ~/.railway/railway.token
```

Then, you can deploy the project:
```
bash ./deploy.sh
```

## Local Development

To run the create-echo-app project locally, you can use [Air](https://github.com/cosmtrek/air) with the environment variables from Railway like so:

```
railway run air
```

This will run the project on `localhost:8080` with the same database as your connected Railway environment.

## Project Structure

create-echo-app is a mimimal starter template for a new echo project, using GORM + PostgreSQL for the backend, and templ + tailwindcss for the frontend.

the main.go file is the entry point for the project, it has a router, an auth middleware, and the controllers for the project.

the models are in types/types.go. The User model we provide is used for authentication by default, and the Post model is used for the blog functionality, and can be optionally deleted.

the templ templates are in templates/ and are used to render the frontend, tailwind is imported via CDN in the head of the layout template.

Happy coding!





