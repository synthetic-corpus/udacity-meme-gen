# Joel Gonzaga’s Meme Generator Project

# Intro

This is a capstone project for Udacity’s Intermediate Python course. Project can be cloned locally ran. It will generate a browse app that create memes with quotes based on user-supplied images and text files.

There is no media content commited in this repo. This project requires images and several text files to work.

There are only two text files commit to this document. Both are found in the _data/miniquotes folder.

>Note: This project works in Python 3.9 or later. Please test accordingly!

## Adding Media Content the Fast Way

If you wish for a quick start, run:  
`Python3 /src/WebEngine/Initializer.py`
This will download a collection of photos and text documents into the _data folder from a public s3 bucket.

Note, that some lines in these test documents will fail to ingest. This is expected because ensuring that things failed expectedly was part of my testing. You also may find that photos are *sideways*. That is an s3 and image metadata problem, but is out of scope of this project.

## Adding Images Manually

To add your own images, place the image file anywhere in the *_data* folder. Valid formats are *.jpg, .jpeg* and *.png*. All other file types will be ignored.

## Adding Text files

Each text file is a collection of quotes and their authors. Valid formats are *.txt*, *.docx*, *.pdf*, and *.csv* and must be specifically formatted.

For docx and pdf each line must be as follows:

`“This is my quote in double quotes” - author name`

The author name must be letters and spaces only. Any characters can go before the single dash. Please take care when writing a .docx file to make sure that the single dash remains *a single dash* because Microsfot Word likes to make a different character. Any other special formatting (e.g. images or tables) will cause the application to break.

.txt must be formatted the same way, but the quotes can be omitted.

.csv files must be formatted like this:

>body,author  
>my fancy quote,author name  
>more inspiring words,smart person

The text files can be placed anywhere in the *_data* folder to be ingested.

## Running For Web Browser

Before running be sure to install all the requirements with pip. Requirements file is requirements.txt. A virtual environment is recommended but not required.

Run the application with:
`python3 src/app.py`
This will load the application which can then be viewed in the browser at localhost:5000. From there, you can click through your random memes, or make a meme with a an image from the web. The web url in this case must end in one of the aforementioned valid image file types.

## Make Memes with CLI

To learn how to make a Meme from the CLI run,  
'Python3 src/meme.py --help'

# Python Modules

The three main folders for modules are MemeEngine, QuoteEngine, and WebEngine.

MemeEngine is responsible for actually creating a meme, and will depend on QuoteEngine and WebEngine.

QuoteEngine is reponsbile for ingest *local* files and turning them into collections of QuoteModel instances. The QuoteModel class is within QuoteEngine.

WebEngine is reponsible for making web request for both text files and image. A short script to initialize this project with text and media data can be found in this module.

# Other Important folders

_data is the folder where both text and image content is expected to be found. Exact locations of the files do not matter, as long as they are in this folder.

The static folder is for finished memes.

And that's it, please enjoy this project!.