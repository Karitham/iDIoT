package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Karitham/iDIoT/api/redis"
	"github.com/Karitham/iDIoT/api/scylla"
	"github.com/Karitham/iDIoT/api/session"
	"github.com/oklog/ulid"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/bcrypt"
)

func DB() *cli.Command {
	return &cli.Command{
		Name:  "db",
		Usage: "Database commands",
		Subcommands: []*cli.Command{
			DBUsers(),
			DBKeys(),
			DBSessions(),
			DBWebpush(),
			DBMigrations(),
			DBReadings(),
		},
	}
}

func DBReadings() *cli.Command {
	return &cli.Command{
		Name:  "readings",
		Usage: "Readings commands",
		Subcommands: []*cli.Command{
			{
				Name:    "list",
				Usage:   "List readings",
				Aliases: []string{"ls"},
				Action: func(c *cli.Context) error {
					s := scylla.New(context.Background(), c.StringSlice("cass")...)
					defer s.Close()

					sensors, err := s.GetSensors(c.Context)
					if err != nil {
						return err
					}

					tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					tw.Write([]byte("Device ID\tDate\tValue\n"))
					for _, r := range sensors {
						fmt.Fprintf(tw, "%s\t%s\t%v\n", r.IoTID, r.Readings[0].Time, r.Readings[0])
					}

					return tw.Flush()
				},
			},
		},
	}
}

func coalesce(args ...string) string {
	for _, a := range args {
		if a != "" {
			return a
		}
	}
	return ""
}

func DBMigrations() *cli.Command {
	return &cli.Command{
		Name:  "migrations",
		Usage: "Migrations commands",
		Subcommands: []*cli.Command{
			{
				Name: "ls",
				Action: func(c *cli.Context) error {
					s := scylla.New(context.Background(), c.StringSlice("cass")...)
					defer s.Close()

					migs, err := s.GetMigrations(c.Context)
					if err != nil {
						return err
					}

					w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
					fmt.Fprintln(w, "ID\tContent\t")
					for _, m := range migs {
						fmt.Fprintf(w, "%d\t%s\n", m.Id, strings.Join(strings.Fields(m.Content), " "))
					}

					return w.Flush()
				},
			},
		},
	}
}

func DBUsers() *cli.Command {
	return &cli.Command{
		Name:    "users",
		Usage:   "Users commands",
		Aliases: []string{"user"},
		Subcommands: []*cli.Command{
			DBUsersAdd(),
			DBUsersList(),
		},
	}
}

func DBUsersAdd() *cli.Command {
	action := func(c *cli.Context) error {
		s := scylla.New(context.Background(), c.StringSlice("cass")...)
		defer s.Close()

		perms := session.Permissions{}
		if c.Bool("admin") {
			perms = append(perms, session.PermRoot)
		}

		pass, err := bcrypt.GenerateFromPassword([]byte(c.String("password")), 12)
		if err != nil {
			return err
		}

		u := scylla.User{
			ID:          ulid.MustNew(ulid.Now(), rand.Reader).String(),
			Name:        c.String("name"),
			Email:       c.String("email"),
			Password:    string(pass),
			Permissions: perms,
		}

		if _, err := s.GetUserByEmail(c.Context, u.Email); err == nil {
			return cli.Exit("User already exists: "+u.Email, 1)
		}

		err = s.CreateUser(c.Context, u)
		if err != nil {
			return err
		}

		log.Info("User created", "id", u.ID)
		return nil
	}

	return &cli.Command{
		Name:   "add",
		Usage:  "Add a user",
		Action: action,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Usage:    "Name",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "email",
				Usage:    "Email",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "password",
				Usage:    "Password",
				Required: true,
			},
			&cli.BoolFlag{
				Name:  "admin",
				Usage: "Admin",
			},
		},
	}
}

func DBSessions() *cli.Command {
	return &cli.Command{
		Name:    "sessions",
		Aliases: []string{"session"},
		Usage:   "Sessions commands",
		Subcommands: []*cli.Command{
			{
				Name:    "list",
				Usage:   "List sessions",
				Aliases: []string{"ls"},
				Action: func(c *cli.Context) error {
					rs, err := redis.New(c.StringSlice("redis-addr"), c.String("redis-user"), c.String("redis-pass"))
					if err != nil {
						return err
					}
					defer rs.Close()

					sessions, err := rs.ListSessions(c.Context)
					if err != nil {
						return err
					}

					tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					tw.Write([]byte("ID\tUser\tPermissions\n"))
					for _, s := range sessions {
						tw.Write([]byte(s.ID.String() + "\t" + s.UserID.String() + "\t" + strings.Join(s.Permissions.Strings(), ",") + "\n"))
					}

					return tw.Flush()
				},
			},
			{
				Name:  "revoke",
				Usage: "Revoke a session",
				Action: func(c *cli.Context) error {
					rs, err := redis.New(c.StringSlice("redis-addr"), c.String("redis-user"), c.String("redis-pass"))
					if err != nil {
						return err
					}
					defer rs.Close()

					id, err := session.Parse([]byte(c.Args().First()))
					if err != nil {
						return err
					}

					err = rs.DeleteSession(c.Context, id)
					if err != nil {
						return err
					}

					log.Info("Session revoked", "id", id)
					return nil
				},
			},
			{
				Name:  "new",
				Usage: "Create a new session",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user",
						Usage:    "User ID",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					s := scylla.New(c.Context, c.StringSlice("cass")...)
					defer s.Close()

					uid := ulid.MustParse(c.String("user"))
					u, err := s.GetUser(c.Context, uid)
					if err != nil {
						return err
					}

					rs, err := redis.New(c.StringSlice("redis-addr"), c.String("redis-user"), c.String("redis-pass"))
					if err != nil {
						return err
					}
					defer rs.Close()

					sess, err := rs.NewSession(c.Context, uid, u.Permissions, time.Hour*24)
					if err != nil {
						return err
					}

					log.Info("Session created", "id", sess.String())
					return nil
				},
			},
		},
	}
}

func DBWebpush() *cli.Command {
	return &cli.Command{
		Name:    "webpush",
		Aliases: []string{"wp"},
		Usage:   "webpush commands",
		Subcommands: []*cli.Command{
			{
				Name:  "send",
				Usage: "Send a webpush message",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user",
						Usage:    "User ID",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "message",
						Usage:    "Message",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					s := scylla.New(c.Context, c.StringSlice("cass")...)
					defer s.Close()

					uid := ulid.MustParse(c.String("user"))
					subs, err := s.GetWebpushSubscriptions(c.Context, uid)
					if err != nil {
						return err
					}

					keys, err := s.GetWebpushKey(c.Context)
					if err != nil {
						return nil
					}

					for _, sub := range subs.Subs {
						err = scylla.SendWebpush(c.Context, keys, sub, []byte(c.String("message")))
						if err != nil {
							log.Error("Failed to send webpush", "err", err)
						}
					}

					return nil
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List wepush subscriptions",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user",
						Usage:    "User ID",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					s := scylla.New(context.Background(), c.StringSlice("cass")...)
					defer s.Close()

					uid := ulid.MustParse(c.String("user"))

					subs, err := s.GetWebpushSubscriptions(c.Context, uid)
					if err != nil {
						return err
					}

					tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					tw.Write([]byte("UserID\tEndpoint\tKey\tAuth\n"))
					for _, s := range subs.Subs {
						fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
							uid,
							s.Endpoint,
							s.Keys.P256dh,
							s.Keys.Auth,
						)
					}

					return tw.Flush()
				},
			},
		},
	}
}

func DBKeys() *cli.Command {
	return &cli.Command{
		Name:    "key",
		Aliases: []string{"keys"},
		Usage:   "interact with keys",
		Subcommands: []*cli.Command{
			{
				Name:  "rotate",
				Usage: "Rotate keys",
				Action: func(c *cli.Context) error {
					s := scylla.New(context.Background(), c.StringSlice("cass")...)
					defer s.Close()

					k, err := s.RotateWebpushKey(c.Context)
					if err != nil {
						return err
					}

					log.Info("Keys rotated", "id", k.ID)
					return nil
				},
			},
			{
				Name:  "get",
				Usage: "Get key",
				Action: func(c *cli.Context) error {
					s := scylla.New(context.Background(), c.StringSlice("cass")...)
					defer s.Close()

					k, err := s.GetWebpushKey(c.Context)
					if err != nil {
						return err
					}

					log.Info("Key", "id", k.ID, "public", k.PublicKey, "private", k.PrivateKey)
					return nil
				},
			},
		},
	}
}

func DBUsersList() *cli.Command {
	action := func(c *cli.Context) error {
		s := scylla.New(context.Background(), c.StringSlice("cass")...)
		defer s.Close()

		users, err := s.GetUsers(c.Context)
		if err != nil {
			return err
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		tw.Write([]byte("ID\tName\tEmail\tPermissions\n"))
		for _, u := range users {
			tw.Write([]byte(u.ID + "\t" + u.Name + "\t" + u.Email + "\t" + strings.Join(u.Permissions.Strings(), ",") + "\n"))
		}

		return tw.Flush()
	}

	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "List users",
		Action:  action,
	}
}
