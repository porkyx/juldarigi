import com.raquo.laminar.api.L.*
import org.scalajs.dom
import org.scalajs.dom.{EventSource, MessageEvent, RequestInit}

import scala.concurrent.Future
import scala.scalajs.concurrent.JSExecutionContext.Implicits.queue
import scala.scalajs.js
import scala.util.Try

object ScraperService {
  // 개발 모드에서는 로컬 서버, 프로덕션에서는 별도 서버 실행 필요
  private val serverUrl = if (dom.window.location.protocol == "file:") {
    // Electron 프로덕션 모드 - 별도 서버 필요
    "http://localhost:4321"
  } else {
    // 개발 모드
    "http://localhost:4321"
  }

  // Domain models
  sealed trait ScrapeMode
  case class PageMode(pages: Int) extends ScrapeMode
  case class DateRangeMode(startDate: Option[String], endDate: Option[String])
      extends ScrapeMode

  case class DCGalleryResult(
      galleryId: String,
      pagesScraped: Int,
      totalPosts: Int,
      uniqueUsers: Int,
      userStats: List[UserStat],
      startDate: Option[String],
      endDate: Option[String]
  )

  case class UserStat(
      uid: String,
      nickname: String,
      ip: String,
      count: Int
  )

  case class ProgressInfo(
      currentPage: Int,
      totalPages: Option[Int],
      totalPosts: Int,
      uniqueUsers: Int,
      message: String
  )

  sealed trait ScrapeEvent
  case class ProgressEvent(info: ProgressInfo) extends ScrapeEvent
  case class CompleteEvent(result: DCGalleryResult) extends ScrapeEvent
  case class ErrorEvent(message: String) extends ScrapeEvent

  // EventBus for streaming events
  private val eventBus = new EventBus[ScrapeEvent]

  // Pure function to build query parameters
  private def buildQueryParams(
      url: String,
      mode: ScrapeMode
  ): dom.URLSearchParams = {
    val params = new dom.URLSearchParams()
    params.append("url", url)
    params.append("type", "dcgallery")

    mode match {
      case PageMode(pages) =>
        params.append("pages", pages.toString)
      case DateRangeMode(startDate, endDate) =>
        startDate.foreach(params.append("startDate", _))
        endDate.foreach(params.append("endDate", _))
    }

    params
  }

  // Stream scraping with EventSource
  def streamScrape(url: String, mode: ScrapeMode): EventStream[ScrapeEvent] = {
    EventStream.fromCustomSource[ScrapeEvent](
      shouldStart = startIndex => startIndex == 1,
      start = (fireEvent, fireError, _, _) => {
        val params = buildQueryParams(url, mode)
        val es = new EventSource(s"$serverUrl/scrape-stream?${params.toString}")

        // Helper to parse JS object safely
        def parseData(data: String): Try[js.Dynamic] =
          Try(js.JSON.parse(data))

        es.addEventListener(
          "start",
          (e: MessageEvent) => {
            fireEvent(
              ProgressEvent(ProgressInfo(0, None, 0, 0, "Starting scraping..."))
            )
          }
        )

        es.addEventListener(
          "progress",
          (e: MessageEvent) => {
            parseData(e.data.asInstanceOf[String]).foreach { data =>
              val currentPage = data.currentPage.asInstanceOf[Int]
              val totalPages = Option(data.totalPages)
                .filter(!js.isUndefined(_))
                .map(_.asInstanceOf[Int])
              val totalPosts = data.totalPosts.asInstanceOf[Int]
              val uniqueUsers = data.uniqueUsers.asInstanceOf[Int]
              val message = data.message.asInstanceOf[String]
              fireEvent(
                ProgressEvent(
                  ProgressInfo(
                    currentPage,
                    totalPages,
                    totalPosts,
                    uniqueUsers,
                    message
                  )
                )
              )
            }
          }
        )

        es.addEventListener(
          "pageComplete",
          (e: MessageEvent) => {
            parseData(e.data.asInstanceOf[String]).foreach { data =>
              val page = data.page.asInstanceOf[Int]
              val postsFound = data.postsFound.asInstanceOf[Int]
              val totalPosts = data.totalPosts.asInstanceOf[Int]
              val uniqueUsers = data.uniqueUsers.asInstanceOf[Int]
              fireEvent(
                ProgressEvent(
                  ProgressInfo(
                    page,
                    None,
                    totalPosts,
                    uniqueUsers,
                    s"Page $page completed: $postsFound posts found"
                  )
                )
              )
            }
          }
        )

        es.addEventListener(
          "info",
          (e: MessageEvent) => {
            parseData(e.data.asInstanceOf[String]).foreach { data =>
              val message = data.message.asInstanceOf[String]
              fireEvent(ProgressEvent(ProgressInfo(0, None, 0, 0, message)))
            }
          }
        )

        es.addEventListener(
          "complete",
          (e: MessageEvent) => {
            parseData(e.data.asInstanceOf[String]).foreach { data =>
              if (data.success.asInstanceOf[Boolean]) {
                val userStatsArray =
                  data.userStats.asInstanceOf[js.Array[js.Dynamic]]
                val userStats = userStatsArray.map { user =>
                  UserStat(
                    uid = user.uid.asInstanceOf[String],
                    nickname = user.nickname.asInstanceOf[String],
                    ip = user.ip.asInstanceOf[String],
                    count = user.count.asInstanceOf[Int]
                  )
                }.toList

                val result = DCGalleryResult(
                  galleryId = data.galleryId.asInstanceOf[String],
                  pagesScraped = data.pagesScraped.asInstanceOf[Int],
                  totalPosts = data.totalPosts.asInstanceOf[Int],
                  uniqueUsers = data.uniqueUsers.asInstanceOf[Int],
                  userStats = userStats,
                  startDate = Option(data.startDate)
                    .map(_.asInstanceOf[String])
                    .filter(_ != null),
                  endDate = Option(data.endDate)
                    .map(_.asInstanceOf[String])
                    .filter(_ != null)
                )
                fireEvent(CompleteEvent(result))
              }
              es.close()
            }
          }
        )

        es.addEventListener(
          "error",
          (e: MessageEvent) => {
            parseData(e.data.asInstanceOf[String]).foreach { data =>
              val errorMsg = data.message.asInstanceOf[String]
              fireEvent(ErrorEvent(errorMsg))
            }
            es.close()
          }
        )

        es.onerror = (_: dom.Event) => {
          if (es.readyState == EventSource.CLOSED) {
            fireEvent(ErrorEvent("Server connection closed"))
          }
        }

        // Return cleanup function
        () => {
          es.close()
        }
      },
      stop = _ => ()
    )
  }

  // Legacy POST-based scraping (for non-streaming)
  def scrapeWithPost(url: String, mode: ScrapeMode): Future[DCGalleryResult] = {
    val bodyObj = js.Dynamic.literal(url = url)

    mode match {
      case PageMode(pages) =>
        bodyObj.pages = pages
      case DateRangeMode(startDate, endDate) =>
        startDate.foreach(d => bodyObj.startDate = d)
        endDate.foreach(d => bodyObj.endDate = d)
    }

    val headers = js.Dictionary(
      "Connection" -> "keep-alive",
      "Content-Type" -> "application/json"
    )

    val requestInit = js.Dynamic
      .literal(
        method = "POST",
        headers = headers,
        body = js.JSON.stringify(bodyObj)
      )
      .asInstanceOf[RequestInit]

    dom
      .fetch(s"$serverUrl/scrape", requestInit)
      .toFuture
      .flatMap(_.json().toFuture)
      .flatMap { json =>
        val data = json.asInstanceOf[js.Dynamic]
        if (
          data.success.asInstanceOf[Boolean] && data
            .selectDynamic("type")
            .asInstanceOf[String] == "dcgallery"
        ) {
          val userStatsArray = data.userStats.asInstanceOf[js.Array[js.Dynamic]]
          val userStats = userStatsArray.map { user =>
            UserStat(
              uid = user.uid.asInstanceOf[String],
              nickname = user.nickname.asInstanceOf[String],
              ip = user.ip.asInstanceOf[String],
              count = user.count.asInstanceOf[Int]
            )
          }.toList

          Future.successful(
            DCGalleryResult(
              galleryId = data.galleryId.asInstanceOf[String],
              pagesScraped = data.pagesScraped.asInstanceOf[Int],
              totalPosts = data.totalPosts.asInstanceOf[Int],
              uniqueUsers = data.uniqueUsers.asInstanceOf[Int],
              userStats = userStats,
              startDate = Option(data.startDate)
                .map(_.asInstanceOf[String])
                .filter(_ != null),
              endDate = Option(data.endDate)
                .map(_.asInstanceOf[String])
                .filter(_ != null)
            )
          )
        } else {
          Future.failed(new Exception(data.error.asInstanceOf[String]))
        }
      }
  }

  // Validate URL
  def isValidDCGalleryUrl(url: String): Boolean =
    url.contains("gall.dcinside.com") && extractGalleryId(url).isDefined

  // Extract gallery ID from URL
  private def extractGalleryId(url: String): Option[String] = {
    val patterns = List(
      """gall\.dcinside\.com/mini/board/lists/?\?id=([^&]+)""".r,
      """gall\.dcinside\.com/mini/([^/?]+)""".r,
      """gall\.dcinside\.com/mgallery/board/lists/?\?id=([^&]+)""".r,
      """gall\.dcinside\.com/mgallery/([^/?]+)""".r,
      """gall\.dcinside\.com/([^/?]+)$""".r
    )

    patterns.view
      .flatMap(_.findFirstMatchIn(url))
      .map(_.group(1))
      .headOption
  }
}
