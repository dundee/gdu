#!/bin/bash

# Script de compilation non invasif pour gdu
# Ce script compile gdu et le place dans un répertoire local/
# qui est ignoré par git pour ne pas interférer avec le dépôt

echo "Compilation de gdu..."

# Créer un répertoire local si nécessaire (ignoré par git)
mkdir -p local

# Compiler gdu et placer le binaire dans le répertoire local
go build -o local/gdu cmd/gdu/main.go

if [ $? -eq 0 ]; then
    echo "✅ Compilation réussie !"
    echo "📍 Le binaire est disponible à : $(pwd)/local/gdu"
    echo ""
    echo "Pour tester :"
    echo "  ./local/gdu --help"
    echo "  ./local/gdu /chemin/vers/repertoire --type yaml,json"
    echo "  ./local/gdu /chemin/vers/repertoire --exclude-type yaml,json"
    echo ""
    echo "💡 Le répertoire local/ est ignoré par git et ne sera pas poussé"
else
    echo "❌ Échec de la compilation"
    exit 1
fi