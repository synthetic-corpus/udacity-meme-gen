# Use Python base image
FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Copy all files from the src directory to the container
COPY . .

# Install dependencies from requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# Expose port 80 (as specified in app.py)
EXPOSE 8080

# Run the Flask application
CMD ["python", "app.py"]

