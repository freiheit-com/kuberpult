
# How to roll back a microservice version

## Concept
You can easily roll back to an older version of a single service

## Rolling back
1) Identify the service and the version you want to deploy.
2) On the homepage, click on the tile representing the service and version you want to roll back.
3) Let's assume it's the tile `Release trains for env groups`: ![](../assets/img/whatsdeployed/overview.png) 
4) Select the environment where you want to deploy, as an example we pick `fakeprod-de`. We click `Deploy & Lock`.
![](../assets/img/rollback/releasedialog-full.png)
5) Now you have 2 planned actions, that you still need to apply. ![](../assets/img/rollback/planned-actions.png)

# Migrations
If you want to use custom migrations you can use [custom_migrations down migration](../database/migrations/postgres/1738234757185160_remove_custom_migrations.down.sql)