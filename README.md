# hxi_bot

Un petit bot d'utilité publique pour le serveur Discord HXi²

Le bot comporte plusieurs "services", qui peuvent etre configurés dans un fichier de configuration json, placé n'importe où, mais par défaut dans `./configs/config.json`.

Ce fichier suit un schéma, defini dans `./config/config.schema.json`; des exemples de configuration sont disponibles dans `./configs`

Le bot est aussi dockerisé, et peut etre lancé (apres avoir ete construit) avec la commande `docker run --rm --name=<NAME> -v ./<CONFIG PATH>:/app/config.json --env-file=.env hxi_bot`
