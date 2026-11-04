FROM python:3.12-slim

# Sortie Python non bufferisée et pas de fichiers .pyc
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

# Les dépendances d'abord pour profiter du cache Docker
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Puis le code
COPY . .

# Utilisateur non root
RUN useradd --create-home --uid 1000 pumpcast
USER pumpcast

CMD ["python", "-m", "pumpcast_ml.sync_external"]
