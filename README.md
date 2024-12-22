# Joel Gonzaga’s Meme Generator Project

# Intro

This project is an expansion of a Udacity capstone project. Please check the ‘udacity-complete’ branch to see the originally completed project.

This project creates a simple web app in which a user can create random memes. Images and files for meme quotes are prepared in an s3 bucket. Then, the project can be deployed with the aws-deploy.yaml file.

The project uses the following Python modules:

- Pillow
- Flask
- Boto3

And several others.

## Project Set Source files

First set up an s3 bucket in the same region you intend to deploy the application. Create the following folders within it

_images
_sources
_textdata
_fonts

At the project's current stage, only _images, sources_, and _fonts can be configured. _textdata is for forthcoming features.

### Set Up Sources Images

Once the s3 bucket has been set up, upload any number .jpeg images into the _sources folder. This will serve as the source for all memes randomly created. _images should remain empty.

### Setting up _fonts
Place any number .ttf fonts in here. This software has only been tested with that format of a font. Memes will be generated from any random font from this part of the folder.

### Adding Text files

Each text file is a collection of quotes and their authors. These will be the randomized quotes for the memes. Valid formats are *.txt*, *.docx*, *.pdf*, and *.csv* and must be specifically formatted.

For docx and pdf each line must be as follows:

`“This is my quote in double quotes” - author name`

The author name must be letters and spaces only. Any characters can go before the single dash. Please take care when writing a .docx file to make sure that the single dash remains *a single dash* because Microsfot Word likes to make a different character. Any other special formatting (e.g. images or tables) will cause the application to break.

.txt must be formatted the same way, but the quotes can be omitted.

.csv files must be formatted like this:

>body,author  
>my fancy quote,author name  
>more inspiring words,smart person

The text files can be placed anywhere in the *_data* folder to be ingested. *You must commit them to your own fork of this repo at this time.*

## Prepare Parameters for Cloud Deployement

The YAML file requires several parameters to set up. Many default values are not expected to work. Those exist for example only.

### SourceS3ARN, SourceS3 and SourceRegion
Set these values to your S3 Bucket ARN and S3 Bucket name respectively. The SourceRegion is self-explanatory and must match where the S3 Bucket was created.

### InstanceType
This field is restricted to the cheapest possible Ec2 instances at this time, but can be expanded if you’re feeling spendy. :-)

### VPCId, SubnetId1, SubnetId2
Set the VPCId to a VPC that has an internet gateway. SubnetIds must be public subnets within that VPC.

### RepoBranch, RepositoryURL
These fields are defaulted to my repo and branch. You can change these to your own fork as needed.

### amiID
This field must be an ami available in the region. See below for further details on this particular ami.

### CWLogGroupName
Whatever you want to log group to be named. Must be unique per region.

## Configuring the AMI
You can use any Linux based AMI. It must have pip3 an at least Python 3.9 installed. Git cloning and pip import handle the rest of the requirements when an ec2 instance is deployed.

# Is the Project working?
Once CW is deployed, you should be able to access it on Load Balancer's public URL on Port 80.

# What is working?
When you make a Meme, it is expected to appear in in the s3 Bucket's _images folder.

# What is yet to be done?
The new Memes will not be visible from a browser yet. That requires additional work.

Creating a Meme from the Web Request URL is not recommended at this time. It will not store the source image or resulting Meme image properly.